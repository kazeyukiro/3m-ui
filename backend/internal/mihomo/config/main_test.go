package config

import (
	"os"
	"testing"
)

// TestMain isolates per-listener TLS material (certstore) to a per-run temp
// directory. Without this, generateListeners reads and writes PEMs keyed by
// listener ID to the production default path (/var/lib/3m-ui/listener-certs).
// Tests that reuse the same listener ID across protocols (e.g. a vless test
// that mints a self-signed cert for ID 1 followed by a shadowsocks test for
// the same ID) would contaminate each other through that shared disk state.
// Pointing 3M_UI_CERT_DIR at a temp dir makes every test hermetic.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "3m-ui-config-test-certs-*")
	if err != nil {
		panic("test cert dir: " + err.Error())
	}
	defer os.RemoveAll(dir)
	os.Setenv("3M_UI_CERT_DIR", dir)
	defer os.Unsetenv("3M_UI_CERT_DIR")
	os.Exit(m.Run())
}
