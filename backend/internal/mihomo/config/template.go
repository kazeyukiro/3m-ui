package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
)

// controllerSecret is a process-scoped random secret applied to Mihomo's
// external-controller. Binding the controller to 127.0.0.1 already keeps it
// off the public network, but without a secret any local user (including a
// compromised less-privileged service running on the same host) could drive
// Mihomo via its REST API. The secret is regenerated on every panel start;
// both the config that we hand to Mihomo and the traffic collector that
// polls Mihomo read this same value via GetDefaultTemplate, so the two
// sides always agree within a single process lifetime.
var (
	controllerSecretOnce sync.Once
	controllerSecret     string
)

// geodataLoaderMemConservative is the core loader built for memory-constrained
// devices. It must be re-applied after user config fragments merge (see
// GenerateFinalConfig) because those fragments overwrite top-level keys and
// could otherwise put the whole GEO dataset back into resident memory.
const geodataLoaderMemConservative = "memconservative"

// ControllerSecret returns the process-scoped external-controller secret,
// generating it on first use. The value is 32 hex chars (128 bits of
// entropy) which matches the strength Mihomo itself recommends.
func ControllerSecret() string {
	controllerSecretOnce.Do(func() {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			// crypto/rand should never fail on Linux. If it does, we still
			// want a non-empty secret rather than silently disabling auth.
			panic("mihomo/config: crypto/rand failed: " + err.Error())
		}
		controllerSecret = hex.EncodeToString(b)
	})
	return controllerSecret
}

// CoreLowMemory reports whether the core should be configured for a
// memory-constrained host (small NAT / VPS boxes without swap).
//
// It is driven by THREE_M_UI_CORE_LOW_MEMORY rather than auto-detected so the
// decision stays visible and reversible: the installer writes it into the
// service unit, and an operator can flip it without rebuilding anything.
// Anything unset or explicitly disabling means "emit no extra keys at all".
func CoreLowMemory() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("THREE_M_UI_CORE_LOW_MEMORY"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetDefaultTemplate returns a deliberately minimal, localhost-safe base
// configuration. Listener definitions are appended from the database.
func GetDefaultTemplate() *MihomoConfig {
	controller := strings.TrimSpace(os.Getenv("THREE_M_UI_MIHOMO_CONTROLLER"))
	if controller == "" {
		controller = "127.0.0.1:9090"
	}
	tmpl := &MihomoConfig{
		Mode:               "rule",
		LogLevel:           "info",
		AllowLan:           false,
		IPv6:               false,
		ExternalController: controller,
		// The controller is bound to loopback, but a process-scoped random
		// secret is still applied so any co-located unprivileged process
		// cannot drive Mihomo's REST API without first reading the secret
		// from this process. Never ship a hard-coded reusable secret.
		Secret: ControllerSecret(),
		DNS: map[string]interface{}{
			"enable": false,
		},
		Proxies:     []map[string]interface{}{},
		ProxyGroups: []map[string]interface{}{},
		Rules: []string{
			// All traffic defaults to DIRECT. A redundant GEOIP rule would
			// require a network download before a clean installation can start.
			"MATCH,DIRECT",
		},
	}
	if CoreLowMemory() {
		tmpl.GeodataLoader = geodataLoaderMemConservative
	}
	return tmpl
}
