package certstore

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Dir returns the directory for durable per-listener TLS material.
// Override with env 3M_UI_CERT_DIR or derive from 3M_UI_DATA / default data root.
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("3M_UI_CERT_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("3M_UI_DATA")); d != "" {
		return filepath.Join(d, "listener-certs")
	}
	return "/var/lib/3m-ui/listener-certs"
}

var mu sync.Mutex

func paths(listenerID uint) (certPath, keyPath string) {
	base := filepath.Join(Dir(), strconv.FormatUint(uint64(listenerID), 10))
	return base + ".crt", base + ".key"
}

// Load returns a previously saved PEM pair for this listener, if present.
func Load(listenerID uint) (certPEM, keyPEM string, ok bool) {
	if listenerID == 0 {
		return "", "", false
	}
	mu.Lock()
	defer mu.Unlock()
	cp, kp := paths(listenerID)
	cb, err1 := os.ReadFile(cp)
	kb, err2 := os.ReadFile(kp)
	if err1 != nil || err2 != nil {
		return "", "", false
	}
	certPEM = string(cb)
	keyPEM = string(kb)
	if strings.TrimSpace(certPEM) == "" || strings.TrimSpace(keyPEM) == "" {
		return "", "", false
	}
	return certPEM, keyPEM, true
}

// Save writes a PEM pair for this listener. Empty inputs are ignored.
func Save(listenerID uint, certPEM, keyPEM string) error {
	if listenerID == 0 {
		return nil
	}
	certPEM = strings.TrimSpace(certPEM)
	keyPEM = strings.TrimSpace(keyPEM)
	if certPEM == "" || keyPEM == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	cp, kp := paths(listenerID)
	if err := os.WriteFile(cp, []byte(certPEM+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(kp, []byte(keyPEM+"\n"), 0o600); err != nil {
		return err
	}
	return nil
}
