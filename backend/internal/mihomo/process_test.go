package mihomo_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
)

func writeFakeMihomo(t *testing.T, dir string) (binaryPath, configPath string) {
	t.Helper()
	binaryPath = filepath.Join(dir, "fake-mihomo")
	configPath = filepath.Join(dir, "config.yaml")

	// Behaves enough like mihomo for ProcessManager:
	//  - `-v` prints a version line and exits
	//  - `-t` validates config and exits 0
	//  - default mode stays running (like a core process)
	script := `#!/bin/sh
case "$1" in
  -v)
    echo "Mihomo Meta v1.0.0 test-build"
    exit 0
    ;;
  -t)
    exit 0
    ;;
esac
exec sleep 30
`
	if err := os.WriteFile(binaryPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("mixed-port: 0\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return binaryPath, configPath
}

func TestProcessManagerStartStop(t *testing.T) {
	dir := t.TempDir()
	binaryPath, configPath := writeFakeMihomo(t, dir)

	// Production only allows /usr/local/bin, /usr/bin, /opt. Tests place a
	// fake binary under t.TempDir(), so temporarily extend the allowlist.
	t.Cleanup(mihomo.AllowBinaryPathPrefixForTesting(dir))

	pm := mihomo.NewProcessManager(binaryPath, configPath)

	status, err := pm.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.Running {
		t.Fatal("expected status.Running to be false initially")
	}

	if err := pm.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	t.Cleanup(func() { _ = pm.Stop() })

	status, err = pm.Status()
	if err != nil {
		t.Fatalf("failed to get status after start: %v", err)
	}
	if !status.Running {
		t.Fatal("expected status.Running to be true after start")
	}
	if status.PID <= 0 {
		t.Fatalf("expected positive PID, got %d", status.PID)
	}

	time.Sleep(1 * time.Second)
	status, err = pm.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.Uptime == "0s" {
		t.Fatal("expected uptime to update from 0s")
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("failed to stop process: %v", err)
	}

	status, err = pm.Status()
	if err != nil {
		t.Fatalf("failed to get status after stop: %v", err)
	}
	if status.Running {
		t.Fatal("expected status.Running to be false after stop")
	}
}

func TestProcessManagerStartMissingBinary(t *testing.T) {
	pm := mihomo.NewProcessManager("/tmp/non_existent_binary_dummy", filepath.Join(t.TempDir(), "config.yaml"))
	err := pm.Start()
	if err == nil {
		t.Fatal("expected start to fail when binary is missing")
	}
}
