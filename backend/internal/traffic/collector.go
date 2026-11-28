package traffic

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo/api"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
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
	// consecutiveMihomoFailures tracks failed CollectOnce cycles. After several
	// failures we clear online flags so the UI does not show stale "online"
	// users while the core is unreachable.
	consecutiveMihomoFailures int
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
		c.mu.Lock()
		c.consecutiveMihomoFailures++
		fails := c.consecutiveMihomoFailures
		c.mu.Unlock()
		// After repeated failures, drop online markers so the dashboard does
		// not keep showing users as online while the core is down.
		if fails >= 3 && c.userSvc != nil {
			if err := c.userSvc.MarkOfflineImmediate(nil); err != nil {
				log.Printf("traffic: clear online after mihomo failures: %v", err)
			}
		}
		return fmt.Errorf("fetch mihomo connections: %w", connErr)
	}
	c.mu.Lock()
	c.consecutiveMihomoFailures = 0
	c.mu.Unlock()

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

	listenerNameToID, listenerIDToName, listenerMultMap, err := c.loadListeners()
	if err != nil {
		return fmt.Errorf("load listeners: %w", err)
	}
	listenerUsers, err := c.loadListenerUsers()
	if err != nil {
		return fmt.Errorf("load listener users: %w", err)
	}
	userListeners := invertListenerUsers(listenerUsers)
	idToName, inboundKeyToID, err := c.loadUserIdentities()
	if err != nil {
		return fmt.Errorf("load proxy user identities: %w", err)
	}

	c.mu.Lock()
	prevConn := c.prevConn
	c.mu.Unlock()

	nextPrev := make(map[string]connSample, len(connResp.Connections))
	views := make([]ConnectionView, 0, len(connResp.Connections))

	type userDelta struct {
		up, down int64 // billable (after multiplier)
	}
	type nodeKey struct {
		uid, lid uint
	}
	userDeltas := make(map[uint]userDelta)
	nodeDeltas := make(map[nodeKey]struct{ up, down int64 })
	listenerMult := listenerMultMap
	if listenerMult == nil {
		listenerMult = make(map[uint]float64)
	}
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
		var network, host, sourceIP, destIP, destPort, inboundUser, inboundName string
		if conn.Metadata != nil {
			network = conn.Metadata.Network
			host = conn.Metadata.Host
			sourceIP = conn.Metadata.SourceIP
			destIP = conn.Metadata.DestinationIP
			destPort = conn.Metadata.DestinationPort
			inboundUser = conn.Metadata.InboundUser
			inboundName = conn.Metadata.InboundName
			if id, ok := resolveListenerID(listenerNameToID, inboundName, destPort); ok {
				lid := id
				listenerID = &lid
				if n, ok := listenerIDToName[lid]; ok {
					listenerName = n
				} else {
					listenerName = inboundName
				}
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

		// Attribution (order of confidence):
		//  1) inboundUser → ProxyUser (UUID / username)
		//  2) single-user listener fallback
		//  3) if user known but node missing: sole bound listener for that user
		//  4) if node known but user missing already handled in (2)
		var proxyUserID *uint
		if uid, ok := resolveProxyUser(inboundKeyToID, inboundUser); ok {
			proxyUserID = &uid
		} else if listenerID != nil {
			if uid, ok := soleUserForListener(listenerUsers, *listenerID); ok {
				proxyUserID = &uid
			}
		}
		// Fill missing node from user's sole binding (VLESS name mismatch recovery).
		if proxyUserID != nil && listenerID == nil {
			if lid, ok := soleListenerForUser(userListeners, *proxyUserID); ok {
				listenerID = &lid
				if n, ok := listenerIDToName[lid]; ok {
					listenerName = n
					view.ListenerName = n
				}
				view.ListenerID = listenerID
			}
		}
		// If both known, prefer counting on that node even when inboundName was fuzzy.

		if proxyUserID != nil {
			view.ProxyUserID = proxyUserID
			view.Username = idToName[*proxyUserID]
			mult := 1.0
			if listenerID != nil {
				if m, ok := listenerMult[*listenerID]; ok {
					mult = m
				}
				nk := nodeKey{uid: *proxyUserID, lid: *listenerID}
				nd := nodeDeltas[nk]
				nd.up += deltaUp
				nd.down += deltaDown
				nodeDeltas[nk] = nd
			}
			d := userDeltas[*proxyUserID]
			d.up += billable(deltaUp, mult)
			d.down += billable(deltaDown, mult)
			userDeltas[*proxyUserID] = d
			if !seenUser[*proxyUserID] {
				seenUser[*proxyUserID] = true
				activeUserIDs = append(activeUserIDs, *proxyUserID)
			}
		}

		views = append(views, view)
	}

	// Group node deltas by user for detailed samples.
	nodesByUser := make(map[uint][]NodeDelta)
	for nk, nd := range nodeDeltas {
		if nd.up == 0 && nd.down == 0 {
			continue
		}
		mult := 1.0
		if m, ok := listenerMult[nk.lid]; ok {
			mult = m
		}
		nodesByUser[nk.uid] = append(nodesByUser[nk.uid], NodeDelta{
			ListenerID: nk.lid, Up: nd.up, Down: nd.down, Multiplier: mult,
		})
	}
	// Persist per-user deltas when traffic moved this tick.
	for uid, d := range userDeltas {
		parts := nodesByUser[uid]
		if d.up == 0 && d.down == 0 && len(parts) == 0 {
			continue
		}
		if err := c.userSvc.AddSampleDetailed(uid, d.up, d.down, parts, true); err != nil {
			log.Printf("traffic: record sample for user %d failed: %v", uid, err)
			continue
		}
	}
	// Mark every user with an attributed connection online, including idle
	// connections with zero byte delta this tick (keep-alive / no traffic yet).
	if err := c.userSvc.MarkOnline(activeUserIDs); err != nil {
		log.Printf("traffic: mark online failed: %v", err)
	}
	for _, uid := range activeUserIDs {
		user.TouchFirstUse(c.db, uid)
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

func (c *Collector) loadListeners() (nameToID map[string]uint, idToName map[uint]string, mult map[uint]float64, err error) {
	var rows []struct {
		ID                uint
		Name              string
		TrafficMultiplier float64
	}
	if err := c.db.Model(&models.Listener{}).Select("id", "name", "traffic_multiplier").Find(&rows).Error; err != nil {
		return nil, nil, nil, err
	}
	nameToID = make(map[string]uint, len(rows)*3)
	idToName = make(map[uint]string, len(rows))
	mult = make(map[uint]float64, len(rows))
	for _, r := range rows {
		idToName[r.ID] = r.Name
		mult[r.ID] = clampMultiplier(r.TrafficMultiplier)
		for _, key := range listenerNameKeys(r.Name) {
			nameToID[key] = r.ID
		}
	}
	return nameToID, idToName, mult, nil
}

// listenerNameKeys returns lookup keys for a panel listener name.
func listenerNameKeys(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	out := []string{name, lower}
	if lower != name {
		out = append(out, lower)
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// resolveListenerID maps Mihomo inboundName to a Listener ID (exact + case-insensitive).
func resolveListenerID(nameToID map[string]uint, inboundName, _ string) (uint, bool) {
	inboundName = strings.TrimSpace(inboundName)
	if inboundName == "" {
		return 0, false
	}
	if id, ok := nameToID[inboundName]; ok {
		return id, true
	}
	if id, ok := nameToID[strings.ToLower(inboundName)]; ok {
		return id, true
	}
	return 0, false
}

// resolveProxyUser maps inboundUser to ProxyUser ID.
func resolveProxyUser(inboundKeyToID map[string]uint, inboundUser string) (uint, bool) {
	key := strings.TrimSpace(inboundUser)
	if key == "" {
		return 0, false
	}
	if id, ok := inboundKeyToID[key]; ok {
		return id, true
	}
	lower := strings.ToLower(key)
	if id, ok := inboundKeyToID[lower]; ok {
		return id, true
	}
	compact := strings.ReplaceAll(lower, "-", "")
	if id, ok := inboundKeyToID[compact]; ok {
		return id, true
	}
	return 0, false
}

// soleUserForListener returns the only bound user when the listener has exactly one.
func soleUserForListener(listenerUsers map[uint][]uint, lid uint) (uint, bool) {
	ids := listenerUsers[lid]
	if len(ids) == 1 {
		return ids[0], true
	}
	return 0, false
}

// soleListenerForUser returns the only bound listener when the user has exactly one.
func soleListenerForUser(userListeners map[uint][]uint, uid uint) (uint, bool) {
	ids := userListeners[uid]
	if len(ids) == 1 {
		return ids[0], true
	}
	return 0, false
}

func invertListenerUsers(listenerUsers map[uint][]uint) map[uint][]uint {
	out := make(map[uint][]uint)
	for lid, uids := range listenerUsers {
		for _, uid := range uids {
			out[uid] = append(out[uid], lid)
		}
	}
	return out
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

// loadUserIdentities returns id→username for display and a reverse map from
// Mihomo inboundUser strings (panel username or UUID) → ProxyUser ID.
func (c *Collector) loadUserIdentities() (idToName map[uint]string, inboundKeyToID map[string]uint, err error) {
	var rows []struct {
		ID       uint
		Username string
		UUID     string
	}
	if err := c.db.Model(&models.ProxyUser{}).Select("id, username, uuid").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	idToName = make(map[uint]string, len(rows))
	inboundKeyToID = make(map[string]uint, len(rows)*3)
	for _, r := range rows {
		idToName[r.ID] = r.Username
		if u := strings.TrimSpace(r.Username); u != "" {
			inboundKeyToID[u] = r.ID
			inboundKeyToID[strings.ToLower(u)] = r.ID
		}
		if id := strings.TrimSpace(r.UUID); id != "" {
			inboundKeyToID[id] = r.ID
			inboundKeyToID[strings.ToLower(id)] = r.ID
			compact := strings.ReplaceAll(strings.ToLower(id), "-", "")
			if compact != "" && compact != strings.ToLower(id) {
				inboundKeyToID[compact] = r.ID
			}
		}
	}
	return idToName, inboundKeyToID, nil
}
