package converter

// clientSubscriptionDocument builds a Mihomo/Clash Meta client document with a
// minimal domestic/international split:
//   - private / CN → DIRECT
//   - everything else → PROXY (manual select, with AUTO url-test + DIRECT)
// Clients need the usual GeoIP/GeoSite databases (shipped with Mihomo/Clash Meta).
func clientSubscriptionDocument(proxies []map[string]interface{}, names []string) map[string]interface{} {
	if names == nil {
		names = []string{}
	}
	selectProxies := make([]string, 0, len(names)+2)
	if len(names) > 0 {
		selectProxies = append(selectProxies, "AUTO")
	}
	selectProxies = append(selectProxies, names...)
	selectProxies = append(selectProxies, "DIRECT")

	groups := []interface{}{
		map[string]interface{}{
			"name":    "PROXY",
			"type":    "select",
			"proxies": selectProxies,
		},
	}
	if len(names) > 0 {
		groups = append(groups, map[string]interface{}{
			"name":     "AUTO",
			"type":     "url-test",
			"proxies":  names,
			"url":      "http://www.gstatic.com/generate_204",
			"interval": 300,
		})
	}

	return map[string]interface{}{
		"mixed-port":  7890,
		"allow-lan":   false,
		"mode":        "rule",
		"log-level":   "info",
		"ipv6":        true,
		"proxies":     proxies,
		"proxy-groups": groups,
		"rules": []string{
			"GEOSITE,private,DIRECT",
			"GEOIP,private,DIRECT,no-resolve",
			"GEOSITE,cn,DIRECT",
			"GEOIP,CN,DIRECT,no-resolve",
			"MATCH,PROXY",
		},
	}
}
