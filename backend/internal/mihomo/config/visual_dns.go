package config

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// Optional booleans retain the distinction between a core default and false.
type VisualDNS struct {
	Enable                       bool                     `json:"enable" yaml:"enable"`
	EnhancedMode                 string                   `json:"enhancedMode,omitempty" yaml:"enhanced-mode,omitempty"`
	Listen                       string                   `json:"listen,omitempty" yaml:"listen,omitempty"`
	IPv6                         *bool                    `json:"ipv6,omitempty" yaml:"ipv6,omitempty"`
	CacheAlgorithm               string                   `json:"cacheAlgorithm,omitempty" yaml:"cache-algorithm,omitempty"`
	PreferH3                     *bool                    `json:"preferH3,omitempty" yaml:"prefer-h3,omitempty"`
	UseHosts                     *bool                    `json:"useHosts,omitempty" yaml:"use-hosts,omitempty"`
	UseSystemHosts               *bool                    `json:"useSystemHosts,omitempty" yaml:"use-system-hosts,omitempty"`
	RespectRules                 *bool                    `json:"respectRules,omitempty" yaml:"respect-rules,omitempty"`
	DefaultNameserver            []string                 `json:"defaultNameserver,omitempty" yaml:"default-nameserver,omitempty"`
	Nameserver                   []string                 `json:"nameserver,omitempty" yaml:"nameserver,omitempty"`
	Fallback                     []string                 `json:"fallback,omitempty" yaml:"fallback,omitempty"`
	ProxyServerNameserver        []string                 `json:"proxyServerNameserver,omitempty" yaml:"proxy-server-nameserver,omitempty"`
	DirectNameserver             []string                 `json:"directNameserver,omitempty" yaml:"direct-nameserver,omitempty"`
	DirectNameserverFollowPolicy *bool                    `json:"directNameserverFollowPolicy,omitempty" yaml:"direct-nameserver-follow-policy,omitempty"`
	NameserverPolicy             map[string]interface{}   `json:"nameserverPolicy,omitempty" yaml:"nameserver-policy,omitempty"`
	ProxyServerNameserverPolicy  map[string]interface{}   `json:"proxyServerNameserverPolicy,omitempty" yaml:"proxy-server-nameserver-policy,omitempty"`
	FakeIPRange                  string                   `json:"fakeIPRange,omitempty" yaml:"fake-ip-range,omitempty"`
	FakeIPRange6                 string                   `json:"fakeIPRange6,omitempty" yaml:"fake-ip-range6,omitempty"`
	FakeIPFilterMode             string                   `json:"fakeIPFilterMode,omitempty" yaml:"fake-ip-filter-mode,omitempty"`
	FakeIPFilter                 []string                 `json:"fakeIPFilter,omitempty" yaml:"fake-ip-filter,omitempty"`
	FakeIPTTL                    *uint32                  `json:"fakeIPTTL,omitempty" yaml:"fake-ip-ttl,omitempty"`
	FallbackFilter               *VisualDNSFallbackFilter `json:"fallbackFilter,omitempty" yaml:"fallback-filter,omitempty"`
	FallbackLazyQuery            *bool                    `json:"fallbackLazyQuery,omitempty" yaml:"fallback-lazy-query,omitempty"`
}

type VisualDNSFallbackFilter struct {
	GeoIP     *bool    `json:"geoip,omitempty" yaml:"geoip,omitempty"`
	GeoIPCode string   `json:"geoipCode,omitempty" yaml:"geoip-code,omitempty"`
	Geosite   []string `json:"geosite,omitempty" yaml:"geosite,omitempty"`
	IPCIDR    []string `json:"ipcidr,omitempty" yaml:"ipcidr,omitempty"`
	Domain    []string `json:"domain,omitempty" yaml:"domain,omitempty"`
}

func (d VisualDNS) validate() error {
	for _, field := range []struct {
		key, value string
		options    []string
	}{
		{"enhanced-mode", d.EnhancedMode, []string{"fake-ip", "redir-host"}},
		{"cache-algorithm", d.CacheAlgorithm, []string{"lru", "arc"}},
		{"fake-ip-filter-mode", d.FakeIPFilterMode, []string{"blacklist", "whitelist", "rule"}},
	} {
		if field.value != "" && !slices.Contains(field.options, field.value) {
			return fmt.Errorf("dns.%s must be one of %s", field.key, strings.Join(field.options, ", "))
		}
	}
	if d.Listen != "" {
		_, port, err := net.SplitHostPort(d.Listen)
		n, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || n < 1 || n > 65535 {
			return fmt.Errorf("dns.listen must be host:port with port between 1 and 65535")
		}
	}
	for _, field := range []struct {
		key, value string
		ipv6       bool
	}{
		{"fake-ip-range", d.FakeIPRange, false},
		{"fake-ip-range6", d.FakeIPRange6, true},
	} {
		if field.value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(field.value)
		if err != nil || prefix.Addr().Is6() != field.ipv6 {
			return fmt.Errorf("dns.%s must be a valid CIDR of the corresponding IP version", field.key)
		}
	}
	for key, policy := range map[string]map[string]interface{}{
		"nameserver-policy": d.NameserverPolicy, "proxy-server-nameserver-policy": d.ProxyServerNameserverPolicy,
	} {
		for domain, value := range policy {
			valid := strings.TrimSpace(domain) != ""
			switch v := value.(type) {
			case string:
				valid = valid && strings.TrimSpace(v) != ""
			case []string:
				valid = valid && len(v) > 0
				for _, server := range v {
					valid = valid && strings.TrimSpace(server) != ""
				}
			case []interface{}:
				valid = valid && len(v) > 0
				for _, server := range v {
					s, ok := server.(string)
					valid = valid && ok && strings.TrimSpace(s) != ""
				}
			default:
				valid = false
			}
			if !valid {
				return fmt.Errorf("dns.%s entries require a domain and a resolver string or nonempty string list", key)
			}
		}
	}
	if d.Enable && d.RespectRules != nil && *d.RespectRules && len(d.ProxyServerNameserver) == 0 {
		return fmt.Errorf("dns.respect-rules requires proxy-server-nameserver")
	}
	if d.FallbackFilter != nil {
		for _, cidr := range d.FallbackFilter.IPCIDR {
			if _, err := netip.ParsePrefix(cidr); err != nil {
				return fmt.Errorf("dns.fallback-filter.ipcidr contains invalid CIDR %q", cidr)
			}
		}
	}
	return nil
}
