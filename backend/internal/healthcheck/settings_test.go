package healthcheck

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.PanelSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.Enabled {
		t.Fatal("monitor must be disabled by default")
	}
	if s.URL != DefaultURL {
		t.Fatalf("url = %q, want %q", s.URL, DefaultURL)
	}
	if s.Interval != 30 || s.Timeout != 10 || s.Failures != 3 || s.Cooldown != 300 {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestNormalizeClampsAndFallsBack(t *testing.T) {
	cases := []struct {
		name string
		in   Settings
		want Settings
	}{
		{
			name: "zero values fall back to defaults",
			in:   Settings{},
			want: DefaultSettings(),
		},
		{
			name: "blank url falls back",
			in:   Settings{URL: "   "},
			want: func() Settings { s := DefaultSettings(); return s }(),
		},
		{
			name: "negative values fall back",
			in:   Settings{URL: "https://x", Interval: -1, Timeout: -1, Failures: 0, Cooldown: -5},
			want: func() Settings {
				s := DefaultSettings()
				s.URL = "https://x"
				return s
			}(),
		},
		{
			name: "valid values preserved and url trimmed",
			in:   Settings{URL: "  https://x  ", Interval: 5, Timeout: 3, Failures: 2, Cooldown: 60, Enabled: true},
			want: Settings{URL: "https://x", Interval: 5, Timeout: 3, Failures: 2, Cooldown: 60, Enabled: true},
		},
		{
			name: "over-max numbers fall back but valid url kept",
			in:   Settings{URL: "https://x", Interval: 999999, Timeout: 99999, Failures: 99999, Cooldown: 999999},
			want: func() Settings {
				s := DefaultSettings()
				s.URL = "https://x"
				return s
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.Normalize()
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	db := openTestDB(t)
	in := Settings{Enabled: true, URL: "https://probe.example/ok", ProxyURL: "socks5://127.0.0.1:1080", Interval: 7, Timeout: 4, Failures: 5, Cooldown: 90}
	if err := SaveSettings(db, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadSettings(db)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != in {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, in)
	}
}

func TestLoadEmptyDBReturnsDefaults(t *testing.T) {
	db := openTestDB(t)
	got, err := LoadSettings(db)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != DefaultSettings() {
		t.Fatalf("got %+v want defaults", got)
	}
}

func TestLoadCorruptRowFallsBackAndErrors(t *testing.T) {
	db := openTestDB(t)
	if err := db.Create(&models.PanelSetting{Key: settingKey, Value: "{not json"}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := LoadSettings(db)
	if err == nil {
		t.Fatal("expected error for corrupt settings row")
	}
}
