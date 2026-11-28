package subpage

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

// cspPolicyTemplate mirrors router.SecurityHeaders' policy. The subscription
// HTML page is a single-file template whose interactive layer is one inline
// <script> block; the global script-src 'self' policy would block it (this
// silently broke the built-in page's copy / i18n / toast since the CSP was
// introduced). InlineScriptCSP re-issues the very same policy with
// per-response sha256 hashes for every inline script block in the rendered
// page, so the single-file template model — built-in or custom theme — keeps
// working without ever allowing 'unsafe-inline'.
const cspPolicyTemplate = "default-src 'self'; " +
	"script-src 'self'%s; " +
	"worker-src 'self' blob:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"object-src 'none'"

var inlineScriptRe = regexp.MustCompile(`(?s)<script(\s[^>]*)?>(.*?)</script>`)

// InlineScriptCSP builds the Content-Security-Policy header value for a
// rendered subscription HTML page: the panel policy plus a sha256 hash for
// each inline <script> block (external scripts stay covered by 'self').
func InlineScriptCSP(html []byte) string {
	hashes := inlineScriptHashes(html)
	if len(hashes) == 0 {
		return fmt.Sprintf(cspPolicyTemplate, "")
	}
	return fmt.Sprintf(cspPolicyTemplate, " "+strings.Join(hashes, " "))
}

func inlineScriptHashes(html []byte) []string {
	var out []string
	for _, m := range inlineScriptRe.FindAllSubmatch(html, -1) {
		if m[1] != nil && strings.Contains(strings.ToLower(string(m[1])), "src=") {
			continue // external script — covered by 'self'
		}
		sum := sha256.Sum256(m[2])
		out = append(out, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return out
}
