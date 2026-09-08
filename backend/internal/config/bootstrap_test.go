package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentEnsureConfigCreatesOneValidFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("THREE_M_UI_DATA_DIR", root)
	path := filepath.Join(root, "config.yaml")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := EnsureConfig(path); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.JWT.Secret) != 64 || len(cfg.Security.CredentialKey) != 64 {
		t.Fatal("invalid secret lengths")
	}
	files, _ := filepath.Glob(filepath.Join(root, ".config-*"))
	if len(files) != 0 {
		t.Fatal("initialization left temporary secrets")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
