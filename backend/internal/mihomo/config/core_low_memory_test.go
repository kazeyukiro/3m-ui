package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// The low-memory core knob must be invisible unless it is switched on: an
// unexpected geodata-loader line changes how every installed core reads its
// GEO data, including cores whose operator never asked for it.
func TestCoreLowMemoryAddsNoKeysByDefault(t *testing.T) {
	t.Setenv("THREE_M_UI_CORE_LOW_MEMORY", "")
	base := mihomoConfig.GetDefaultTemplate()
	if base.GeodataLoader != "" {
		t.Fatalf("expected no geodata loader override, got %q", base.GeodataLoader)
	}
	raw, err := yaml.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "geodata-loader") {
		t.Fatalf("unexpected key in default template:\n%s", raw)
	}
}

func TestCoreLowMemorySelectsMemConservativeLoader(t *testing.T) {
	for _, enabled := range []string{"1", "true", "YES", "on"} {
		t.Run(enabled, func(t *testing.T) {
			t.Setenv("THREE_M_UI_CORE_LOW_MEMORY", enabled)
			base := mihomoConfig.GetDefaultTemplate()
			if base.GeodataLoader != "memconservative" {
				t.Fatalf("expected memconservative loader, got %q", base.GeodataLoader)
			}
		})
	}
}

// Anything that is not an explicit enable must stay off. A typo in an operator's
// unit file should degrade to stock behaviour, never silently switch loaders.
func TestCoreLowMemoryRejectsAmbiguousValues(t *testing.T) {
	for _, disabled := range []string{"0", "false", "no", "off", "maybe"} {
		t.Run(disabled, func(t *testing.T) {
			t.Setenv("THREE_M_UI_CORE_LOW_MEMORY", disabled)
			if got := mihomoConfig.GetDefaultTemplate().GeodataLoader; got != "" {
				t.Fatalf("value %q should disable the knob, got %q", disabled, got)
			}
		})
	}
}

// User config fragments merge wholesale over the template, so a fragment
// carrying geodata-loader: standard would otherwise undo the tuning on the
// exact machines that need it most.
func TestGenerateFinalConfigPinsLoaderAgainstFragments(t *testing.T) {
	cases := []struct {
		name             string
		lowMemory        string
		wantLoader       string
		wantStandardGone bool
	}{
		{"low memory overrides standard", "1", "memconservative", true},
		{"fragments win when knob is off", "", "standard", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("THREE_M_UI_CORE_LOW_MEMORY", tt.lowMemory)
			db := lowMemoryTestDB(t)
			fragment := models.Config{
				Name:    "operator-tuned-core",
				Type:    "custom",
				Enabled: true,
				Content: "geodata-loader: standard\n",
			}
			if err := db.Create(&fragment).Error; err != nil {
				t.Fatalf("seed fragment: %v", err)
			}
			out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
			if err != nil {
				t.Fatalf("generate config: %v", err)
			}
			if !strings.Contains(out, "geodata-loader: "+tt.wantLoader) {
				t.Fatalf("expected loader %q in generated config:\n%s", tt.wantLoader, out)
			}
			if tt.wantStandardGone && strings.Contains(out, "geodata-loader: standard") {
				t.Fatalf("operator fragment leaked the standard loader back in:\n%s", out)
			}
			// The knob must not disturb traffic routing defaults.
			if !strings.Contains(out, "MATCH,DIRECT") {
				t.Fatalf("default DIRECT rule lost:\n%s", out)
			}
		})
	}
}

func lowMemoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "3m-ui-lowmem.db")
	db, err := database.InitDB(path)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = os.Remove(path)
	})
	return db
}
