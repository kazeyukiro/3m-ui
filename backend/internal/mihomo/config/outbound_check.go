package config

import (
	"fmt"
	"slices"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"gorm.io/gorm"
)

// OutboundCheckProfile contains only the selected saved outbound and its dependencies.
// It is consumed server-side; it can contain proxy credentials and WARP keys.
type OutboundCheckProfile struct {
	Enabled bool
	Proxies []map[string]any
	Groups  []map[string]any
}

func ServerOutboundCheckProfile(db *gorm.DB, name string) (OutboundCheckProfile, error) {
	profile := OutboundCheckProfile{}
	sr, err := GetServerRouting(db)
	if err != nil {
		return profile, err
	}
	acc, err := system.GetWARPAccount(db)
	if err != nil {
		return profile, err
	}
	proxies := map[string]map[string]any{}
	for _, value := range proxyEntriesToYAML(sr.Proxies) {
		p := value.(map[string]any)
		proxies[p["name"].(string)] = p
	}
	if acc != nil {
		// Obtain the managed definition on the server, never from the redacted list API.
		delete(proxies, system.WARPProxyName)
	}
	if _, exists := proxies[name]; !exists && !(name == system.WARPProxyName && acc != nil) {
		return profile, ErrOutboundNotFound
	}
	if slices.Contains(sr.DisabledOutboundNames, name) && !(name == system.WARPProxyName && acc != nil) {
		return profile, nil
	}
	groups := map[string]map[string]any{}
	for _, value := range groupEntriesToYAML(sr.Groups) {
		g := value.(map[string]any)
		groups[g["name"].(string)] = g
	}
	visiting, included := map[string]bool{}, map[string]bool{}
	var include func(string) error
	include = func(dependency string) error {
		if slices.Contains([]string{"DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE"}, dependency) || included[dependency] {
			return nil
		}
		if visiting[dependency] {
			return fmt.Errorf("%w: cyclic dialer dependency", ErrInvalidOutbound)
		}
		visiting[dependency] = true
		p, exists := proxies[dependency]
		if dependency == system.WARPProxyName && acc != nil {
			p, err = acc.ProxyMap()
			if err != nil {
				return err
			}
			exists = true
		} else if slices.Contains(sr.DisabledOutboundNames, dependency) {
			return fmt.Errorf("%w: a dialer dependency is disabled", ErrInvalidOutbound)
		}
		if exists {
			profile.Proxies = append(profile.Proxies, p)
			if dialer, _ := p["dialer-proxy"].(string); dialer != "" {
				if err := include(dialer); err != nil {
					return err
				}
			}
		} else if g, ok := groups[dependency]; ok {
			profile.Groups = append(profile.Groups, g)
			members, _ := g["proxies"].([]string)
			for _, member := range members {
				if err := include(member); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("%w: a dialer dependency is missing", ErrInvalidOutbound)
		}
		visiting[dependency], included[dependency] = false, true
		return nil
	}
	if err := include(name); err != nil {
		return OutboundCheckProfile{}, err
	}
	profile.Enabled = true
	return profile, nil
}
