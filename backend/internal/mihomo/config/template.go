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

// GetDefaultTemplate returns a deliberately minimal, localhost-safe base
// configuration. Listener definitions are appended from the database.
func GetDefaultTemplate() *MihomoConfig {
	controller := strings.TrimSpace(os.Getenv("THREE_M_UI_MIHOMO_CONTROLLER"))
	if controller == "" {
		controller = "127.0.0.1:9090"
	}
	return &MihomoConfig{
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
}
