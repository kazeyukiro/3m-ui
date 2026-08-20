package system

import (
	"fmt"
	"strings"
)

// WARPTemplate returns a Mihomo YAML fragment for Cloudflare WARP (WireGuard)
// outbound — 3x-ui "WARP" helper parity. Operators paste private_key / addresses
// from `warp-cli` or wgcf.
func WARPTemplate(privateKey, address, reserved string) string {
	privateKey = strings.TrimSpace(privateKey)
	address = strings.TrimSpace(address)
	if address == "" {
		address = "172.16.0.2/32"
	}
	reserved = strings.TrimSpace(reserved)
	reservedLine := ""
	if reserved != "" {
		reservedLine = fmt.Sprintf("\n  reserved: [%s]", reserved)
	}
	keyLine := privateKey
	if keyLine == "" {
		keyLine = "YOUR_WARP_PRIVATE_KEY"
	}
	return fmt.Sprintf(`# Cloudflare WARP outbound for Mihomo (paste into config / routing)
proxies:
  - name: WARP
    type: wireguard
    private-key: %s
    server: engage.cloudflareclient.com
    port: 2408
    ip: %s
    public-key: bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
    udp: true%s
    mtu: 1280

proxy-groups:
  - name: WARP-OUT
    type: select
    proxies:
      - WARP
      - DIRECT

# Example rule (optional):
# rules:
#   - MATCH,WARP-OUT
`, keyLine, strings.Split(address, "/")[0], reservedLine)
}
