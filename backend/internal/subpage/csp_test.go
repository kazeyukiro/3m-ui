package subpage

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestInlineScriptCSPHashesInlineBlock(t *testing.T) {
	body := "document.querySelector('.x').addEventListener('click', function(){ alert('你好'); });"
	html := []byte("<!doctype html><html><head><script>" + body + "</script></head><body></body></html>")

	csp := InlineScriptCSP(html)
	if !strings.Contains(csp, "script-src 'self' 'sha256-") {
		t.Fatalf("inline script not hashed into policy: %s", csp)
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("script-src must not allow unsafe-inline: %s", csp)
	}
	sum := sha256.Sum256([]byte(body))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(csp, want) {
		t.Fatalf("hash mismatch: want %s in %s", want, csp)
	}
	// Other directives must survive untouched.
	for _, want := range []string{
		"default-src 'self'", "style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:", "frame-ancestors 'none'", "object-src 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("directive %q missing: %s", want, csp)
		}
	}
}

func TestInlineScriptCSPSkipsExternalScripts(t *testing.T) {
	html := []byte(`<script src="/assets/app.js"></script><script>console.log(1)</script>`)
	csp := InlineScriptCSP(html)
	if strings.Count(csp, "sha256-") != 1 {
		t.Fatalf("only the inline block should be hashed: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self' 'sha256-") {
		t.Fatalf("external src script should stay covered by 'self': %s", csp)
	}
}

func TestInlineScriptCSPNoScripts(t *testing.T) {
	html := []byte("<!doctype html><html><body><p>plain</p></body></html>")
	csp := InlineScriptCSP(html)
	if strings.Contains(csp, "sha256-") {
		t.Fatalf("no inline scripts should produce no hashes: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self';") {
		t.Fatalf("base policy must fall back to script-src 'self'; : %s", csp)
	}
}

func TestInlineScriptCSPMatchesRenderedPage(t *testing.T) {
	// The real page carries exactly one inline script; the header produced
	// for the rendered bytes must match the hash of that script body.
	page := renderTestPage(t)
	start := strings.Index(string(page), "<script>")
	end := strings.Index(string(page), "</script>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("rendered page has no inline <script> block")
	}
	body := string(page)[start+len("<script>") : end]
	sum := sha256.Sum256([]byte(body))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	csp := InlineScriptCSP(page)
	if !strings.Contains(csp, want) {
		t.Fatalf("rendered-page script hash missing from CSP: %s", csp)
	}
}
