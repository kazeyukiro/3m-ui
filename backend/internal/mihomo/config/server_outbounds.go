package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"gorm.io/gorm"
)

var (
	ErrInvalidOutbound    = errors.New("invalid outbound")
	ErrOutboundNotFound   = errors.New("outbound not found")
	ErrOutboundReferenced = errors.New("outbound is referenced")
)

type ServerOutbound struct {
	Proxy   ProxyEntry `json:"proxy"`
	Enabled bool       `json:"enabled"`
	Managed bool       `json:"managed"`
}

var outboundRequired = map[string][]string{
	"ss": {"cipher", "password"}, "snell": {"psk"},
	"vmess": {"uuid", "cipher"}, "vless": {"uuid"}, "trojan": {"password"},
	"hysteria2": {}, "tuic": {}, "shadowquic": {}, "anytls": {"password"},
	"mieru": {"username", "password", "transport"}, "sudoku": {"key"},
	"trusttunnel": {}, "http": {}, "socks5": {}, "wireguard": {"private-key"},
}

func ListServerOutbounds(db *gorm.DB) ([]ServerOutbound, error) {
	sr, err := GetServerRouting(db)
	if err != nil {
		return nil, err
	}
	entries := make([]ServerOutbound, 0, len(sr.Proxies)+1)
	acc, err := system.GetWARPAccount(db)
	if err != nil {
		return nil, err
	}
	for _, p := range sr.Proxies {
		if acc != nil && p.Name == system.WARPProxyName {
			continue
		}
		entries = append(entries, ServerOutbound{Proxy: p, Enabled: !slices.Contains(sr.DisabledOutboundNames, p.Name)})
	}
	if acc != nil {
		// The list needs no WARP private key or Cloudflare account token.
		entries = append(entries, ServerOutbound{Proxy: ProxyEntry{Name: system.WARPProxyName, Type: "wireguard", Server: acc.EndpointHost, Port: acc.EndpointPort}, Enabled: true, Managed: true})
	}
	return entries, nil
}

func validateOutbound(p ProxyEntry, sr ServerRoutingConfig, previous string) error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrInvalidOutbound, message) }
	if p.Name == "" || strings.TrimSpace(p.Name) != p.Name || strings.ContainsAny(p.Name, ",\r\n") {
		return invalid("name must be non-empty, trimmed, and contain no comma or newline")
	}
	if slices.Contains([]string{"DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE", "GLOBAL", "WARP"}, strings.ToUpper(p.Name)) {
		return invalid("name is reserved: " + p.Name)
	}
	for _, other := range sr.Proxies {
		if other.Name == p.Name && other.Name != previous {
			return invalid("name already exists: " + p.Name)
		}
	}
	for _, group := range sr.Groups {
		if group.Name == p.Name {
			return invalid("name conflicts with server group: " + p.Name)
		}
	}
	required, supported := outboundRequired[p.Type]
	if !supported {
		return invalid("unsupported protocol: " + p.Type)
	}
	// WireGuard supports either one root peer or a list of peers.
	peers, multiPeer := p.Options["peers"].([]interface{})
	if p.Type == "wireguard" && multiPeer && len(peers) > 0 {
		for _, peer := range peers {
			m, ok := peer.(map[string]interface{})
			if !ok {
				return invalid("invalid WireGuard peer")
			}
			if err := validateOutboundEndpoint(m["server"], m["port"]); err != nil {
				return invalid(err.Error())
			}
			if strings.TrimSpace(fmt.Sprint(m["public-key"])) == "" || m["public-key"] == nil {
				return invalid("WireGuard peers require public-key")
			}
		}
	} else {
		port := p.Port
		if (p.Type == "hysteria2" && p.Options["ports"] != nil) || (p.Type == "mieru" && p.Options["port-range"] != nil) {
			port = 1
		}
		if err := validateOutboundEndpoint(p.Server, port); err != nil {
			return invalid(err.Error())
		}
		if p.Type == "wireguard" {
			required = append(slices.Clone(required), "public-key")
		}
	}
	for _, key := range required {
		value, ok := p.Options[key].(string)
		if !ok || value == "" {
			return invalid(key + " is required")
		}
	}
	optionString := func(key string) string { value, _ := p.Options[key].(string); return value }
	if p.Type == "tuic" && optionString("token") == "" && (optionString("uuid") == "" || optionString("password") == "") {
		return invalid("TUIC requires a v4 token or v5 uuid and password")
	}
	if p.Type == "wireguard" && optionString("ip") == "" && optionString("ipv6") == "" {
		return invalid("WireGuard requires ip or ipv6")
	}
	if p.Options["dialer-proxy"] == p.Name {
		return invalid("dialer-proxy cannot reference itself")
	}
	for _, key := range []string{"enabled", "managed", "protocol", "disabledOutboundNames"} {
		if _, present := p.Options[key]; present {
			return invalid(key + " is panel metadata, not a proxy option")
		}
	}
	return nil
}

func rejectManagedWARP(db *gorm.DB, name string) error {
	if name != system.WARPProxyName {
		return nil
	}
	acc, err := system.GetWARPAccount(db)
	if err != nil {
		return err
	}
	if acc != nil {
		return fmt.Errorf("%w: WARP is managed in Settings", ErrInvalidOutbound)
	}
	return nil
}

func validateOutboundEndpoint(server, port interface{}) error {
	host, ok := server.(string)
	if !ok || strings.TrimSpace(host) == "" || strings.ContainsAny(host, "\r\n\t /") {
		return errors.New("a valid server host is required")
	}
	n, err := strconv.Atoi(fmt.Sprint(port))
	if err != nil || n < 1 || n > 65535 {
		return errors.New("port must be an integer between 1 and 65535")
	}
	return nil
}

// SaveServerOutbound creates (previous == "") or updates one panel-owned proxy.
// Renames and their references are committed with the definition in one transaction.
func SaveServerOutbound(db *gorm.DB, previous string, entry ServerOutbound) error {
	return db.Transaction(func(tx *gorm.DB) error {
		sr, err := GetServerRouting(tx)
		if err != nil {
			return err
		}
		if err := rejectManagedWARP(tx, previous); err != nil {
			return err
		}
		idx := slices.IndexFunc(sr.Proxies, func(p ProxyEntry) bool { return p.Name == previous })
		if previous != "" && idx < 0 {
			return ErrOutboundNotFound
		}
		if err := validateOutbound(entry.Proxy, sr, previous); err != nil {
			return err
		}
		if previous != "" && !entry.Enabled {
			if refs := outboundReferences(sr, previous); len(refs) > 0 {
				return referenceError(previous, refs)
			}
		}
		if idx < 0 {
			sr.Proxies = append(sr.Proxies, entry.Proxy)
		} else {
			sr.Proxies[idx] = entry.Proxy
		}
		if previous != "" && previous != entry.Proxy.Name {
			renameOutboundReferences(&sr, previous, entry.Proxy.Name)
		}
		sr.DisabledOutboundNames = slices.DeleteFunc(sr.DisabledOutboundNames, func(n string) bool { return n == previous || n == entry.Proxy.Name })
		if !entry.Enabled {
			sr.DisabledOutboundNames = append(sr.DisabledOutboundNames, entry.Proxy.Name)
		}
		if err := validateActiveOutboundReferences(tx, sr); err != nil {
			return err
		}
		return SaveServerRouting(tx, sr)
	})
}

func DeleteServerOutbound(db *gorm.DB, name string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := rejectManagedWARP(tx, name); err != nil {
			return err
		}
		sr, err := GetServerRouting(tx)
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(sr.Proxies, func(p ProxyEntry) bool { return p.Name == name })
		if idx < 0 {
			return ErrOutboundNotFound
		}
		if refs := outboundReferences(sr, name); len(refs) > 0 {
			return referenceError(name, refs)
		}
		sr.Proxies = slices.Delete(sr.Proxies, idx, idx+1)
		sr.DisabledOutboundNames = slices.DeleteFunc(sr.DisabledOutboundNames, func(n string) bool { return n == name })
		return SaveServerRouting(tx, sr)
	})
}

func SetServerOutboundsEnabled(db *gorm.DB, names []string, enabled bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		sr, err := GetServerRouting(tx)
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := rejectManagedWARP(tx, name); err != nil {
				return err
			}
			if !slices.ContainsFunc(sr.Proxies, func(p ProxyEntry) bool { return p.Name == name }) {
				return ErrOutboundNotFound
			}
			if !enabled {
				if refs := outboundReferences(sr, name); len(refs) > 0 {
					return referenceError(name, refs)
				}
			}
			sr.DisabledOutboundNames = slices.DeleteFunc(sr.DisabledOutboundNames, func(n string) bool { return n == name })
			if !enabled {
				sr.DisabledOutboundNames = append(sr.DisabledOutboundNames, name)
			}
		}
		if err := validateActiveOutboundReferences(tx, sr); err != nil {
			return err
		}
		return SaveServerRouting(tx, sr)
	})
}

func ruleTargetIndex(parts []string) int {
	if len(parts) < 2 {
		return -1
	}
	tp := strings.ToUpper(strings.TrimSpace(parts[0]))
	if tp == "SUB-RULE" {
		return -1
	}
	if tp == "MATCH" {
		return 1
	}
	if len(parts) < 3 {
		return -1
	}
	if ruleCommaPayloadTypes[tp] {
		return len(parts) - 1
	}
	return 2
}

// Visit only routing targets, never a matching hostname, regex or rule payload.
func visitOutboundReferences(sr *ServerRoutingConfig, visit func(string, string) string) {
	for i, rule := range sr.Rules {
		parts := strings.Split(rule, ",")
		idx := ruleTargetIndex(parts)
		if idx >= 0 {
			target := strings.TrimSpace(parts[idx])
			if replacement := visit(target, fmt.Sprintf("rule %d", i+1)); replacement != target {
				parts[idx] = replacement
				sr.Rules[i] = strings.Join(parts, ",")
			}
		}
	}
	for i := range sr.Groups {
		for j, target := range sr.Groups[i].Proxies {
			sr.Groups[i].Proxies[j] = visit(target, "group "+sr.Groups[i].Name)
		}
	}
	for i := range sr.RuleProviders {
		if sr.RuleProviders[i].Proxy != "" {
			sr.RuleProviders[i].Proxy = visit(sr.RuleProviders[i].Proxy, "rule-provider "+sr.RuleProviders[i].Name)
		}
	}
	for i := range sr.Proxies {
		if target, ok := sr.Proxies[i].Options["dialer-proxy"].(string); ok && target != "" {
			sr.Proxies[i].Options["dialer-proxy"] = visit(target, "dialer-proxy of "+sr.Proxies[i].Name)
		}
	}
}

func outboundReferences(sr ServerRoutingConfig, name string) []string {
	refs := []string{}
	visitOutboundReferences(&sr, func(target, location string) string {
		if target == name {
			refs = append(refs, location)
		}
		return target
	})
	if name == system.WARPProxyName {
		if sr.WarpGlobal {
			refs = append(refs, "global WARP")
		}
		if len(sr.WarpDomains) > 0 {
			refs = append(refs, "WARP domains")
		}
	}
	return refs
}

func renameOutboundReferences(sr *ServerRoutingConfig, old, name string) {
	visitOutboundReferences(sr, func(target, _ string) string {
		if target == old {
			return name
		}
		return target
	})
}

func referenceError(name string, refs []string) error {
	return fmt.Errorf("%w: %s — remove references first: %s", ErrOutboundReferenced, name, strings.Join(refs, ", "))
}

func validateActiveOutboundReferences(db *gorm.DB, sr ServerRoutingConfig) error {
	for _, name := range sr.DisabledOutboundNames {
		if refs := outboundReferences(sr, name); len(refs) > 0 {
			return referenceError(name, refs)
		}
	}
	acc, err := system.GetWARPAccount(db)
	if err != nil {
		return err
	}
	if acc == nil && !slices.ContainsFunc(sr.Proxies, func(p ProxyEntry) bool { return p.Name == system.WARPProxyName }) {
		if refs := outboundReferences(sr, system.WARPProxyName); len(refs) > 0 {
			return referenceError("WARP (not configured)", refs)
		}
	}
	return nil
}

// DeleteServerWARP keeps the dependency check and account deletion atomic.
func DeleteServerWARP(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		acc, err := system.GetWARPAccount(tx)
		if err != nil {
			return err
		}
		if acc == nil {
			return nil
		}
		sr, err := GetServerRouting(tx)
		if err != nil {
			return err
		}
		if refs := outboundReferences(sr, system.WARPProxyName); len(refs) > 0 {
			return fmt.Errorf("%w: %w", system.ErrWARPInUse, referenceError(system.WARPProxyName, refs))
		}
		// Older server-routing records may contain a same-name WARP entry
		// shadowed by the Settings account. Do not resurrect it on deletion.
		sr.Proxies = slices.DeleteFunc(sr.Proxies, func(p ProxyEntry) bool { return p.Name == system.WARPProxyName })
		sr.DisabledOutboundNames = slices.DeleteFunc(sr.DisabledOutboundNames, func(name string) bool { return name == system.WARPProxyName })
		if err := SaveServerRouting(tx, sr); err != nil {
			return err
		}
		return system.DeleteWARPAccount(tx)
	})
}
