package node

import (
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
)

// normalizeExportHost picks a public hostname for share links.
func normalizeExportHost(requestHost, bind, listen string) string {
	return normalizeExportHostPrefer(requestHost, bind, listen, "")
}

// normalizeExportHostPrefer priority: publicURL > request Host > bind/listen.
// Loopback is only used as a last resort so local panels can still export.
// Wildcard binds (0.0.0.0 / ::) are skipped; bare IPv6 is returned without brackets.
func normalizeExportHostPrefer(requestHost, bind, listen, publicURL string) string {
	candidates := []string{}
	if publicURL != "" {
		candidates = append(candidates, publicURL)
	}
	candidates = append(candidates, requestHost, bind, listen)

	var loopback string
	for _, raw := range candidates {
		h := netutil.NormalizeHost(raw)
		if h == "" || netutil.IsUnspecifiedBind(h) {
			continue
		}
		if netutil.IsLoopbackHost(h) {
			if loopback == "" {
				loopback = h
			}
			continue
		}
		return h
	}
	return loopback
}
