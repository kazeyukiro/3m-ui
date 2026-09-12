package converter

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const maxExternalSubBytes = 2 << 20 // 2 MiB

// mergeExternalSubscriptionLinks fetches optional Clash/Mihomo YAML URLs and
// appends their proxies into the user subscription document.
func mergeExternalSubscriptionLinks(rawLinks string, proxies []map[string]interface{}, names []string) ([]map[string]interface{}, []string) {
	client := &http.Client{Timeout: 8 * time.Second}
	for _, line := range strings.Split(rawLinks, "\n") {
		u := strings.TrimSpace(line)
		if u == "" || strings.HasPrefix(u, "#") {
			continue
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "3m-ui-external-sub/1.0")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxExternalSubBytes+1))
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode >= 400 || len(body) > maxExternalSubBytes {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(body, &doc); err != nil {
			continue
		}
		list, _ := doc["proxies"].([]interface{})
		for i, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("ext-%d", i+1)
				m["name"] = name
			}
			// Avoid colliding with local proxy names.
			base := name
			n := 2
			for containsName(names, name) {
				name = fmt.Sprintf("%s-%d", base, n)
				n++
				m["name"] = name
			}
			proxies = append(proxies, m)
			names = append(names, name)
		}
	}
	return proxies, names
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
