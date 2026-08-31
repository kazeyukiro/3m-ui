package traffic

import (
	"fmt"
	"log"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo/api"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

// Collector periodically polls Mihomo's external controller API
// (/traffic and /connections) and turns the result into:
//   - a global traffic Snapshot (traffic.Service)
//   - TrafficRecord history + ProxyUser counters (traffic.UserService)
//   - an in-memory, API-exposable view of current connections
//
// Connection -> ProxyUser attribution follows the existing architecture
// exactly (Connection -> Listener -> ListenerUser -> ProxyUser) and never
// invents a new permission model. When a connection cannot be attributed
// without guessing, it is kept as unknown (ListenerID/ProxyUserID nil).
type Collector struct {
	db      *gorm.DB
	client  *api.Client
	svc     *Service
	userSvc *UserService

	mu               sync.Mutex
	prevConn         map[string]connSample
	lastConnections  []ConnectionView
	lastListenerName map[uint]string // listener ID -> name, cached for API consumers
}

type connSample struct {
	upload   int64
	download int64
}

// NewCollector builds a Collector. baseURL/secret should point at Mihomo's
// external-controller (see mihomo/config.GetDefaultTemplate for the
// defaults 3m-ui writes into the generated Mihomo config).
func NewCollector(db *gorm.DB, baseURL, secret string, svc *Service, userSvc *UserService) *Collector {
	return &Collector{
		db:       db,
		client:   api.NewClient(baseURL, secret),
		svc:      svc,
		userSvc:  userSvc,
		prevConn: make(map[string]connSample),
	}
}

// NewCollectorFromDefaults builds a Collector using the same
// external-controller address/secret the Config Engine writes into
// Mihomo's own config (mihomo/config.GetDefaultTemplate), so the collector
// talks to the same instance 3m-ui manages without introducing a second
// place to configure the controller address.
func NewCollectorFromDefaults(db *gorm.DB, svc *Service, userSvc *UserService) *Collector {
	tmpl := mihomoConfig.GetDefaultTemplate()
	return NewCollector(db, "http://"+tmpl.ExternalController, tmpl.Secret, svc, userSvc)
}

// CollectOnce performs a single collection cycle: fetch, map, and persist.
// Errors talking to Mihomo (e.g. core not running) are returned but are
// expected to be non-fatal to the caller (the scheduler logs and retries
// on the next tick).
func (c *Collector) CollectOnce() error {
	connResp, connErr := c.client.Connections()
	if connErr != nil {
		return fmt.Errorf("fetch mihomo connections: %w", connErr)
	}

	// /traffic gives an instantaneous up/down rate sample (bytes in the
	// last second) directly from Mihomo, which is more accurate than
	// deriving a rate from two /connections polls 10s apart. It's treated
	// as optional: if unavailable, the global snapshot just omits the rate
	// refinement and callers fall back to Service's own delta calculation.
	var upRate, downRate int64
	trafficAvailable := false
	if t, err := c.client.Traffic(); err == nil {
		upRate, downRate = t.Up, t.Down
		trafficAvailable = true
	}

	listenerNameToID, listenerIDToName, err := c.loadListeners()
	if err != nil {
		return fmt.Errorf("load listeners: %w", err)
	}
	listenerUsers, err := c.loadListenerUsers()
	if err != nil {
		return fmt.Errorf("load listener users: %w", err)
	}
	usernames, err := c.loadUsernames()
	if err != nil {
		return fmt.Errorf("load proxy usernames: %w", err)
	}

	c.mu.Lock()
	prevConn := c.prevConn
	c.mu.Unlock()

	nextPrev := make(map[string]connSample, len(connResp.Connections))
	views := make([]ConnectionView, 0, len(connResp.Connections))

	type userDelta struct {
		up, down int64
	}
	userDeltas := make(map[uint]userDelta)
	activeUserIDs := make([]uint, 0)
	seenUser := make(map[uint]bool)

	for _, conn := range connResp.Connections {
		prev := prevConn[conn.ID]
		deltaUp := conn.Upload - prev.upload
		deltaDown := conn.Download - prev.download
		if deltaUp < 0 {
			deltaUp = conn.Upload // counter reset or first sighting mid-connection
		}
		if deltaDown < 0 {
			deltaDown = conn.Download
		}
		nextPrev[conn.ID] = connSample{upload: conn.Upload, download: conn.Download}

		view := ConnectionView{
			ID:       conn.ID,
			Upload:   conn.Upload,
			Download: conn.Download,
		}

		var listenerID *uint
		var listenerName string
		var network, host, sourceIP, destIP, destPort, inboundUser string
		if conn.Metadata != nil {
			network = conn.Metadata.Network
			host = conn.Metadata.Host
			sourceIP = conn.Metadata.SourceIP
			destIP = conn.Metadata.DestinationIP
			destPort = conn.Metadata.DestinationPort
			inboundUser = conn.Metadata.InboundUser
			if id, ok := listenerNameToID[conn.Metadata.InboundName]; ok {
				listenerID = &id
				listenerName = conn.Metadata.InboundName
			}
		}
		if network == "" {
			network = conn.Network // fall back to deprecated top-level field
		}
		view.Network = network
		view.Host = host
		view.SourceIP = sourceIP
		view.DestinationIP = destIP
		view.DestinationPort = destPort
		view.ListenerID = listenerID
		view.ListenerName = listenerName
		view.Rule = conn.Rule
		view.Chains = conn.Chains
		view.Start = conn.Start

		// Attribution, in order of confidence. Never guess: if neither
		// path applies, the connection stays unattributed.
		var proxyUserID *uint
		if inboundUser != "" {
			// Mihomo told us the authenticated username directly.
			for id, name := range usernames {
				if name == inboundUser {
					uid := id
					proxyUserID = &uid
					break
				}
			}
		} else if listenerID != nil {
			// Fall back: only attribute when the listener has exactly one
			// bound ProxyUser, since with more than one we cannot tell
			// which of them this connection belongs to.
			if ids := listenerUsers[*listenerID]; len(ids) == 1 {
				uid := ids[0]
				proxyUserID = &uid
			}
		}

		if proxyUserID != nil {
			view.ProxyUserID = proxyUserID
			view.Username = usernames[*proxyUserID]
			d := userDeltas[*proxyUserID]
			d.up += deltaUp
			d.down += deltaDown
			userDeltas[*proxyUserID] = d
			if !seenUser[*proxyUserID] {
				seenUser[*proxyUserID] = true
				activeUserIDs = append(activeUserIDs, *proxyUserID)
			}
		}

		views = append(views, view)
	}

	// Persist per-user deltas and refresh online/last-seen state.
	for uid, d := range userDeltas {
		if d.up == 0 && d.down == 0 {
			continue
		}
		if err := c.userSvc.AddSample(uid, d.up, d.down, true); err != nil {
			// Don't abort the whole collection cycle for one user's DB error:
			// other users' deltas would be lost and the global snapshot below
			// would never run, leaving the dashboard stale. Log via the wrapped
			// error and keep going so the remaining users are still processed.
			log.Printf("traffic: record sample for user %d failed: %v", uid, err)
			continue
		}
	}
	if err := c.userSvc.MarkOffline(activeUserIDs); err != nil {
		return fmt.Errorf("update online status: %w", err)
	}

	if trafficAvailable {
		c.svc.ApplySample(connResp.UploadTotal, connResp.DownloadTotal, len(connResp.Connections), upRate, downRate)
	} else {
		// Mihomo's /traffic endpoint is optional/unavailable on some setups.
		// Fall back to the cumulative counters so the dashboard still gets a
		// useful rate instead of being stuck at 0 B/s.
		c.svc.Update(connResp.UploadTotal, connResp.DownloadTotal, len(connResp.Connections))
	}

	c.mu.Lock()
	c.prevConn = nextPrev
	c.lastConnections = views
	c.lastListenerName = listenerIDToName
	c.mu.Unlock()

	return nil
}

// CurrentConnections returns the most recently mapped connection snapshot.
func (c *Collector) CurrentConnections() []ConnectionView {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ConnectionView, len(c.lastConnections))
	copy(out, c.lastConnections)
	return out
}

func (c *Collector) loadListeners() (nameToID map[string]uint, idToName map[uint]string, err error) {
	var rows []struct {
		ID   uint
		Name string
	}
	if err := c.db.Model(&models.Listener{}).Select("id, name").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	nameToID = make(map[string]uint, len(rows))
	idToName = make(map[uint]string, len(rows))
	for _, r := range rows {
		nameToID[r.Name] = r.ID
		idToName[r.ID] = r.Name
	}
	return nameToID, idToName, nil
}

// loadListenerUsers returns, for each Listener ID, the ProxyUser IDs bound
// to it via ListenerUser -- the existing join table. No new relationship is
// introduced.
func (c *Collector) loadListenerUsers() (map[uint][]uint, error) {
	var rows []struct {
		ListenerID  uint
		ProxyUserID uint
	}
	if err := c.db.Model(&models.ListenerUser{}).Select("listener_id, proxy_user_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint][]uint)
	for _, r := range rows {
		out[r.ListenerID] = append(out[r.ListenerID], r.ProxyUserID)
	}
	return out, nil
}

func (c *Collector) loadUsernames() (map[uint]string, error) {
	var rows []struct {
		ID       uint
		Username string
	}
	if err := c.db.Model(&models.ProxyUser{}).Select("id, username").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Username
	}
	return out, nil
}
