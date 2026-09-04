package converter

import (
	"encoding/json"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// loadBoundRemoteMirrors returns enabled remote node mirrors bound to the user.
func loadBoundRemoteMirrors(db *gorm.DB, userID uint) ([]models.RemoteNodeMirror, error) {
	var binds []models.ProxyUserRemoteNode
	if err := db.Where("proxy_user_id = ?", userID).Find(&binds).Error; err != nil {
		return nil, err
	}
	if len(binds) == 0 {
		return nil, nil
	}
	ids := make([]uint, 0, len(binds))
	for _, b := range binds {
		ids = append(ids, b.RemoteNodeMirrorID)
	}
	var mirrors []models.RemoteNodeMirror
	if err := db.Where("id IN ? AND enabled = ?", ids, true).Find(&mirrors).Error; err != nil {
		return nil, err
	}
	return mirrors, nil
}

// appendRemoteProxyMaps extracts Mihomo proxy maps from mirrored ClientYAML / URI.
func appendRemoteProxyMaps(mirrors []models.RemoteNodeMirror, allProxies []map[string]interface{}, names []string) ([]map[string]interface{}, []string) {
	for _, m := range mirrors {
		proxies := proxiesFromClientYAML(m.ClientYAML, m.Name, m.RemoteServerID)
		if len(proxies) == 0 && strings.TrimSpace(m.ShareURI) != "" {
			// URI-only nodes cannot become typed maps without a parser; skip YAML merge.
			continue
		}
		for _, p := range proxies {
			n, _ := p["name"].(string)
			if n == "" {
				n = m.Name
				p["name"] = n
			}
			// Prefix to avoid name collisions with local nodes.
			prefix := "R" + itoaU(m.RemoteServerID) + "-"
			if !strings.HasPrefix(n, prefix) {
				n = prefix + n
				p["name"] = n
			}
			allProxies = append(allProxies, p)
			names = append(names, n)
		}
	}
	return allProxies, names
}

func proxiesFromClientYAML(raw, fallbackName string, remoteServerID uint) []map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		// Maybe a single proxy map
		var one map[string]interface{}
		if err2 := yaml.Unmarshal([]byte(raw), &one); err2 == nil && one["type"] != nil {
			if one["name"] == nil || one["name"] == "" {
				one["name"] = fallbackName
			}
			return []map[string]interface{}{one}
		}
		return nil
	}
	if pl, ok := doc["proxies"].([]interface{}); ok {
		out := make([]map[string]interface{}, 0, len(pl))
		for _, item := range pl {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	if doc["type"] != nil {
		if doc["name"] == nil || doc["name"] == "" {
			doc["name"] = fallbackName
		}
		return []map[string]interface{}{doc}
	}
	return nil
}

func appendRemoteShareURIs(mirrors []models.RemoteNodeMirror, uris []string) []string {
	for _, m := range mirrors {
		if u := strings.TrimSpace(m.ShareURI); u != "" {
			uris = append(uris, u)
		}
		if strings.TrimSpace(m.ShareURIsJSON) != "" {
			var extra []string
			if json.Unmarshal([]byte(m.ShareURIsJSON), &extra) == nil {
				for _, u := range extra {
					u = strings.TrimSpace(u)
					if u != "" && u != strings.TrimSpace(m.ShareURI) {
						uris = append(uris, u)
					}
				}
			}
		}
	}
	return uris
}

func itoaU(n uint) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
