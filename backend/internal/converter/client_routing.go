package converter

import (
	"fmt"
	"strings"

	mihomocfg "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

// clientSubscriptionDocument builds Mihomo/Clash Meta **client** YAML.
//
// When the panel has custom routing (proxy-groups / non-default rules in
// visual-config), those are exported for the client — not applied as server
// outbound policy for subscription users. Leaf groups that only list DIRECT
// get subscription node names injected so selects work on the phone/PC.
//
// Fallback (no custom routing): PROXY select + AUTO url-test + CN direct rules.
func clientSubscriptionDocument(proxies []map[string]interface{}, names []string, visual *mihomocfg.VisualConfig) map[string]interface{} {
	if names == nil {
		names = []string{}
	}

	useCustom := visual != nil && hasClientRouting(visual)
	var groups []interface{}
	var rules []string

	// Merge panel visual outbounds (e.g. visual outbounds) into the client document
	// so optional visual outbounds for client YAML. Server strips visual proxies.
	if visual != nil && len(visual.Proxies) > 0 {
		proxies, names = mergeVisualProxiesForClient(proxies, names, visual.Proxies)
	}

	if useCustom {
		groups = adaptVisualGroupsForClient(visual.Groups, names)
		rules = filterClientRules(visual.Rules, groupNamesFromAdapted(groups), names)
		if len(rules) == 0 {
			rules = defaultClientRules()
		}
		// Ensure every node is reachable from some top-level path.
		if len(names) > 0 && !groupsReferenceAnyProxy(groups, names) {
			sel := append([]string{"AUTO"}, names...)
			sel = append(sel, "DIRECT")
			groups = append([]interface{}{
				map[string]interface{}{
					"name":    "PROXY",
					"type":    "select",
					"proxies": sel,
				},
				map[string]interface{}{
					"name":     "AUTO",
					"type":     "url-test",
					"proxies":  names,
					"url":      "http://www.gstatic.com/generate_204",
					"interval": 300,
				},
			}, groups...)
		}
	} else {
		groups = defaultClientGroups(names)
		rules = defaultClientRules()
	}

	for i := range proxies {
		sanitizeClientProxyMap(proxies[i])
	}
	if visual != nil {
		applyClientSubscriptionDialFlags(proxies, visual)
	}

	return map[string]interface{}{
		"mixed-port":   7890,
		"allow-lan":    false,
		"mode":         "rule",
		"log-level":    "info",
		"ipv6":         true,
		"proxies":      proxies,
		"proxy-groups": groups,
		"rules":        rules,
	}
}

func defaultClientRules() []string {
	return []string{
		"GEOSITE,private,DIRECT",
		"GEOIP,private,DIRECT,no-resolve",
		"GEOSITE,cn,DIRECT",
		"GEOIP,CN,DIRECT,no-resolve",
		"MATCH,PROXY",
	}
}

func defaultClientGroups(names []string) []interface{} {
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
	return groups
}

func hasClientRouting(v *mihomocfg.VisualConfig) bool {
	if len(v.Proxies) > 0 {
		return true
	}
	if len(v.Groups) > 0 {
		return true
	}
	if len(v.Rules) == 0 {
		return false
	}
	// Default server inbound is a single MATCH,DIRECT — treat as "no client layout".
	if len(v.Rules) == 1 {
		r := strings.ToUpper(strings.TrimSpace(v.Rules[0]))
		if r == "MATCH,DIRECT" || r == "MATCH, DIRECT" {
			return false
		}
	}
	return true
}

func adaptVisualGroupsForClient(src []mihomocfg.GroupEntry, names []string) []interface{} {
	groupNameSet := make(map[string]struct{}, len(src))
	for _, g := range src {
		n := strings.TrimSpace(g.Name)
		if n != "" {
			groupNameSet[n] = struct{}{}
		}
	}
	proxySet := make(map[string]struct{}, len(names))
	for _, n := range names {
		proxySet[n] = struct{}{}
	}

	out := make([]interface{}, 0, len(src)+2)
	for _, g := range src {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			continue
		}
		typ := strings.TrimSpace(g.Type)
		if typ == "" {
			typ = "select"
		}
		members := make([]string, 0, len(g.Proxies)+len(names))
		for _, m := range g.Proxies {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if m == "DIRECT" || m == "REJECT" || m == "COMPATIBLE" {
				members = append(members, m)
				continue
			}
			if _, ok := groupNameSet[m]; ok {
				members = append(members, m)
				continue
			}
			if _, ok := proxySet[m]; ok {
				members = append(members, m)
				continue
			}
			// Drop outbounds not present in this subscription.
		}
		if shouldInjectSubscriptionNodes(name, members, groupNameSet) && len(names) > 0 {
			injected := make([]string, 0, len(names)+len(members))
			injected = append(injected, names...)
			for _, m := range members {
				if m == "DIRECT" || m == "REJECT" || m == "COMPATIBLE" {
					injected = append(injected, m)
					continue
				}
				if _, ok := groupNameSet[m]; ok {
					injected = append(injected, m)
				}
			}
			members = uniqueStrings(injected)
		}
		if len(members) == 0 {
			members = []string{"DIRECT"}
		}
		entry := map[string]interface{}{
			"name":    name,
			"type":    typ,
			"proxies": members,
		}
		if (typ == "url-test" || typ == "fallback" || typ == "load-balance") && strings.TrimSpace(g.URL) != "" {
			entry["url"] = g.URL
			iv := g.Interval
			if iv <= 0 {
				iv = 300
			}
			entry["interval"] = iv
		}
		out = append(out, entry)
	}
	return out
}

// Leaf selects that only had DIRECT (community templates) get real nodes.
// Policy groups that only reference other groups are left alone.
func shouldInjectSubscriptionNodes(groupName string, members []string, groupNameSet map[string]struct{}) bool {
	lower := strings.ToLower(strings.TrimSpace(groupName))
	if lower == "直连" || strings.Contains(lower, "adblock") || lower == "reject" {
		return false
	}
	hasGroupRef := false
	hasOnlyBuiltin := true
	for _, m := range members {
		if _, ok := groupNameSet[m]; ok {
			hasGroupRef = true
			hasOnlyBuiltin = false
			continue
		}
		if m != "DIRECT" && m != "REJECT" && m != "COMPATIBLE" {
			hasOnlyBuiltin = false
		}
	}
	if hasGroupRef {
		return false
	}
	return hasOnlyBuiltin || len(members) == 0
}

func filterClientRules(rules []string, groupNames map[string]struct{}, proxyNames []string) []string {
	valid := map[string]struct{}{"DIRECT": {}, "REJECT": {}, "COMPATIBLE": {}}
	for n := range groupNames {
		valid[n] = struct{}{}
	}
	for _, n := range proxyNames {
		valid[n] = struct{}{}
	}
	out := make([]string, 0, len(rules))
	for _, line := range rules {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		targetIdx := len(parts) - 1
		target := strings.TrimSpace(parts[targetIdx])
		if strings.EqualFold(target, "no-resolve") && len(parts) >= 3 {
			targetIdx = len(parts) - 2
			target = strings.TrimSpace(parts[targetIdx])
		}
		if _, ok := valid[target]; !ok {
			// Unknown outbound — fall back to PROXY group or DIRECT.
			if _, has := valid["PROXY"]; has {
				parts[targetIdx] = "PROXY"
			} else if _, has := valid["代理"]; has {
				parts[targetIdx] = "代理"
			} else {
				parts[targetIdx] = "DIRECT"
			}
			line = strings.Join(parts, ",")
		}
		out = append(out, line)
	}
	return out
}

func groupNamesFromAdapted(groups []interface{}) map[string]struct{} {
	m := make(map[string]struct{})
	for _, g := range groups {
		mm, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		if n, _ := mm["name"].(string); n != "" {
			m[n] = struct{}{}
		}
	}
	return m
}

func groupsReferenceAnyProxy(groups []interface{}, names []string) bool {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	for _, g := range groups {
		mm, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		list, _ := mm["proxies"].([]string)
		if list == nil {
			if raw, ok := mm["proxies"].([]interface{}); ok {
				for _, x := range raw {
					s, _ := x.(string)
					if _, ok := set[s]; ok {
						return true
					}
				}
			}
			continue
		}
		for _, s := range list {
			if _, ok := set[s]; ok {
				return true
			}
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func mergeVisualProxiesForClient(
	proxies []map[string]interface{},
	names []string,
	visualProxies []mihomocfg.ProxyEntry,
) ([]map[string]interface{}, []string) {
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}
	for _, vp := range visualProxies {
		n := strings.TrimSpace(vp.Name)
		if n == "" || strings.TrimSpace(vp.Type) == "" {
			continue
		}
		base := n
		i := 2
		for {
			if _, ok := nameSet[n]; !ok {
				break
			}
			n = fmt.Sprintf("%s-%d", base, i)
			i++
		}
		m := map[string]interface{}{
			"name":   n,
			"type":   vp.Type,
			"server": vp.Server,
			"port":   vp.Port,
		}
		for k, v := range vp.Options {
			if isServerOnlyProxyKey(k) {
				continue
			}
			m[k] = v
		}
		// Normalise TFO/MPTCP: client proxies use bool `tfo` / `mptcp` (wiki proxies#tfo).
		// Never emit inbound-tfo / inbound-mptcp (those are server general keys only).
		normalizeClientDialFlags(m)
		proxies = append(proxies, m)
		names = append(names, n)
		nameSet[n] = struct{}{}
	}
	return proxies, names
}

// Server-side / general keys that must never appear on a client proxy entry.
func isServerOnlyProxyKey(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "inbound-tfo", "inbound-mptcp", "inboundtfo", "inboundmptcp",
		"listeners", "users", "user", "account", "accounts",
		"certificate", "private-key", "private_key", "privatekey",
		"client-auth-cert", "client-auth-key":
		return true
	default:
		return false
	}
}

func normalizeClientDialFlags(m map[string]interface{}) {
	if m == nil {
		return
	}
	// Drop server general dial keys if someone stuffed them into options.
	delete(m, "inbound-tfo")
	delete(m, "inbound-mptcp")
	delete(m, "inboundTfo")
	delete(m, "inboundMptcp")
	// Wiki: tfo / mptcp are booleans on the proxy object. Coerce truthy; omit false.
	for _, key := range []string{"tfo", "mptcp"} {
		v, ok := m[key]
		if !ok {
			continue
		}
		if asBool(v) {
			m[key] = true
		} else {
			delete(m, key)
		}
	}
}

func sanitizeClientProxyMap(m map[string]interface{}) {
	if m == nil {
		return
	}
	for k := range m {
		if isServerOnlyProxyKey(k) {
			delete(m, k)
		}
	}
	normalizeClientDialFlags(m)
}

func asBool(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}

func applyClientSubscriptionDialFlags(proxies []map[string]interface{}, visual *mihomocfg.VisualConfig) {
	if visual == nil {
		return
	}
	for _, m := range proxies {
		if m == nil {
			continue
		}
		if visual.ClientTfo {
			m["tfo"] = true
		}
		if visual.ClientMPTCP {
			m["mptcp"] = true
		}
		normalizeClientDialFlags(m)
	}
}
