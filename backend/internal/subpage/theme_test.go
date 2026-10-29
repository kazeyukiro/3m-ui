package subpage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestLoadPageSettingsDefaultTheme(t *testing.T) {
	s := LoadPageSettings(nil)
	if s.Theme != "blue" {
		t.Fatalf("default theme = %q, want blue", s.Theme)
	}
}

// RenderHTML with a nil DB (default settings) must render the blue theme:
// blue gradient background, blue accent, and the supplied username / base URL.
func TestRenderHTMLBlueTheme(t *testing.T) {
	pu := models.ProxyUser{
		Username:     "testuser",
		Enabled:      true,
		TrafficUsed:  42 * 1024 * 1024 * 1024,
		TrafficLimit: 200 * 1024 * 1024 * 1024,
		SubToken:     "t",
	}
	base := "https://h.example.com/api/v1/client/sub/t"
	html, err := RenderHTML(nil, pu, base, []string{"vless://u@h:443?security=tls#HK-01"})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	out := string(html)
	for _, must := range []string{
		"testuser",
		"linear-gradient(160deg, #4aa8f5",
		"#2f9df4",
		base,
	} {
		if !strings.Contains(out, must) {
			t.Errorf("blue theme missing %q", must)
		}
	}
	// The dark theme's background must not leak into the blue theme. The toast
	// keeps a dark *text* color (#0e1420) on its white pill, so only assert that
	// no dark *background* usage survives.
	if strings.Contains(out, "background: #0e1420") || strings.Contains(out, "var(--bg, #0e1420)") {
		t.Errorf("blue theme still contains dark background #0e1420")
	}
}

// loadTemplate selects the built-in theme by the theme id and falls through to
// blue for unknown values.
func TestLoadTemplateTheme(t *testing.T) {
	blue, err := loadTemplate("", "blue")
	if err != nil {
		t.Fatalf("loadTemplate blue: %v", err)
	}
	bb := bytes.Buffer{}
	_ = blue.Execute(&bb, ViewModel{Username: "x"})
	if !strings.Contains(bb.String(), "linear-gradient(160deg, #4aa8f5") &&
		!strings.Contains(bb.String(), "#2f9df4") {
		t.Errorf("blue template missing blue markers")
	}

	classic, err := loadTemplate("", "classic")
	if err != nil {
		t.Fatalf("loadTemplate classic: %v", err)
	}
	cb := bytes.Buffer{}
	_ = classic.Execute(&cb, ViewModel{Username: "x"})
	if !strings.Contains(cb.String(), "#0e1420") {
		t.Errorf("classic template missing dark marker #0e1420")
	}

	// Unknown theme id falls back to blue.
	fallback, err := loadTemplate("", "bogus")
	if err != nil {
		t.Fatalf("loadTemplate bogus: %v", err)
	}
	fb := bytes.Buffer{}
	_ = fallback.Execute(&fb, ViewModel{Username: "x"})
	if !strings.Contains(fb.String(), "linear-gradient(160deg, #4aa8f5") {
		t.Errorf("unknown theme did not fall back to blue")
	}
}
