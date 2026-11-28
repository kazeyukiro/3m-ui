package traffic

import (
	"log"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Enforcer watches for proxy users that cross into (or out of) a blocked
// state -- disabled, expired, or TrafficUsed >= TrafficLimit -- and, when
// that changes, triggers a Mihomo config regeneration so the exclusion
// takes effect promptly instead of waiting for the next manual node edit.
//
// The block/allow decision itself is never duplicated here: it is made
// exclusively by user.IsCredentialActive, the same predicate
// user.Service.ActiveCredentialsByListener uses when building the
// credentials that go into the generated Mihomo config. Enforcer only
// detects when that predicate's result changed for any user.
type Enforcer struct {
	db      *gorm.DB
	nodeSvc *node.Service

	mu          sync.Mutex
	lastBlocked map[uint]bool
}

// NewEnforcer builds an Enforcer. nodeSvc is used only to call
// RegenerateConfig() (which itself calls
// user.Service.ActiveCredentialsByListener() and attempts a Mihomo hot
// reload) -- no additional filtering logic lives here.
func NewEnforcer(db *gorm.DB, nodeSvc *node.Service) *Enforcer {
	return &Enforcer{db: db, nodeSvc: nodeSvc, lastBlocked: map[uint]bool{}}
}

// CheckAndEnforce recomputes the blocked set and regenerates + hot-reloads
// the Mihomo config only if it changed since the last check. Returns the
// number of currently blocked users for callers that want to report it
// (e.g. the dashboard), and any error encountered.
func (e *Enforcer) CheckAndEnforce() (blockedCount int, err error) {
	var users []models.ProxyUser
	if err := e.db.Find(&users).Error; err != nil {
		return 0, err
	}

	current := make(map[uint]bool, len(users))
	for _, u := range users {
		if !user.IsCredentialActive(u) {
			current[u.ID] = true
		}
	}

	// Detect change against the last successfully-enforced state WITHOUT
	// updating lastBlocked yet. If we updated lastBlocked first and
	// RegenerateConfig failed, the next tick would see changed==false and
	// never retry — leaving over-quota users connected. Instead, we keep the
	// old state on failure so the next tick re-detects the same diff.
	e.mu.Lock()
	changed := !equalBlockedSets(e.lastBlocked, current)
	e.mu.Unlock()

	if !changed {
		return len(current), nil
	}

	log.Printf("traffic: enforcement state changed (%d user(s) now blocked); regenerating Mihomo config", len(current))
	if e.nodeSvc == nil {
		// Nothing to regenerate; record state so we don't keep logging.
		e.mu.Lock()
		e.lastBlocked = current
		e.mu.Unlock()
		return len(current), nil
	}
	if err := e.nodeSvc.RegenerateConfig(); err != nil {
		// Leave lastBlocked as-is so the next tick detects the same diff
		// and retries regen. Over-quota users must not stay connected due
		// to a single transient regen failure.
		return len(current), err
	}
	e.mu.Lock()
	e.lastBlocked = current
	// Prune entries for users no longer present in the latest snapshot (e.g.
	// deleted proxy users) so lastBlocked does not grow without bound if a
	// future code path starts mutating lastBlocked incrementally. Today the
	// whole map is replaced above, so this loop is a defensive no-op — but it
	// is done under the lock to avoid a data race on the map (delete while
	// another goroutine reads via CheckAndEnforce).
	for id := range e.lastBlocked {
		if _, ok := current[id]; !ok {
			delete(e.lastBlocked, id)
		}
	}
	e.mu.Unlock()
	return len(current), nil
}

func equalBlockedSets(a, b map[uint]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
