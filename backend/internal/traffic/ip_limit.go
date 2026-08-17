package traffic

import (
	"log"
	"sort"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// EnforceIPLimits closes excess connections when a ProxyUser exceeds ip_limit
// distinct source IPs (0 = unlimited).
func (c *Collector) EnforceIPLimits() {
	if c == nil || c.db == nil || c.client == nil {
		return
	}
	c.mu.Lock()
	views := append([]ConnectionView(nil), c.lastConnections...)
	c.mu.Unlock()
	if len(views) == 0 {
		return
	}

	var users []models.ProxyUser
	if err := c.db.Select("id", "username", "ip_limit", "enabled").Find(&users).Error; err != nil {
		log.Printf("traffic: load users for ip limit: %v", err)
		return
	}
	limitByUser := map[uint]int{}
	for _, u := range users {
		if u.Enabled && u.IPLimit > 0 {
			limitByUser[u.ID] = u.IPLimit
		}
	}
	if len(limitByUser) == 0 {
		return
	}

	type ipConns struct {
		ip  string
		ids []string
	}
	byUser := map[uint]map[string][]string{}
	for _, v := range views {
		if v.ProxyUserID == nil {
			continue
		}
		uid := *v.ProxyUserID
		if _, ok := limitByUser[uid]; !ok {
			continue
		}
		ip := v.SourceIP
		if ip == "" {
			ip = "unknown"
		}
		if byUser[uid] == nil {
			byUser[uid] = map[string][]string{}
		}
		byUser[uid][ip] = append(byUser[uid][ip], v.ID)
	}

	closed := 0
	for uid, ips := range byUser {
		limit := limitByUser[uid]
		if len(ips) <= limit {
			continue
		}
		list := make([]ipConns, 0, len(ips))
		for ip, ids := range ips {
			list = append(list, ipConns{ip: ip, ids: ids})
		}
		sort.Slice(list, func(i, j int) bool {
			if len(list[i].ids) != len(list[j].ids) {
				return len(list[i].ids) > len(list[j].ids)
			}
			return list[i].ip < list[j].ip
		})
		for i := limit; i < len(list); i++ {
			for _, id := range list[i].ids {
				if err := c.client.CloseConnection(id); err != nil {
					log.Printf("traffic: close conn %s (user %d ip %s): %v", id, uid, list[i].ip, err)
					continue
				}
				closed++
			}
		}
	}
	if closed > 0 {
		log.Printf("traffic: ip_limit enforcement closed %d connection(s)", closed)
	}
}
