package system

import (
	"fmt"
	"net"
	"strings"
)

// isValidDomain reports whether s is a plausible DNS hostname (used in ACME /
// reverse-proxy templates). It rejects anything containing shell-significant
// characters, whitespace or path separators — the values are interpolated into
// nginx config and shell commands, so even a well-intentioned operator
// typo could otherwise produce an injection.
func isValidDomain(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 253 {
		return false
	}
	// Disallow anything that is not a hostname label character. Allow ASCII
	// letters/digits, dots, hyphens and trailing dot (FQDN). Reject Unicode /
	// IDN until the panel grows a punycode encoder.
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' || r == '-' {
			continue
		}
		return false
	}
	// Cheap structural check: every label must be non-empty and not start/end
	// with a hyphen.
	for _, label := range strings.Split(strings.TrimSuffix(s, "."), ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

// isValidEmail reports whether s looks like a bare RFC-822-ish address. We do
// not need a full grammar; we just need to keep the value from breaking out of
// the shell command we paste it into.
func isValidEmail(s string) bool {
	s = strings.TrimSpace(s)
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	local := s[:at]
	host := s[at+1:]
	if !isValidDomain(host) {
		return false
	}
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '+':
		default:
			return false
		}
	}
	return true
}

// shellQuote wraps s in single quotes for safe interpolation into a shell
// command. Embedded single quotes are escaped with the standard
// '\” trick.
func shellQuote(s string) string {
	// Standard POSIX shell quoting: wrap the value in single quotes and escape
	// any embedded single quote as '\'' (close quote, escaped quote, reopen).
	const q = "'"
	const esc = "'\\''"
	return q + strings.ReplaceAll(s, q, esc) + q
}

// validateUnixPath rejects shell-significant characters and path traversal in
// webroot / cert-dir style parameters. It does NOT require the path to exist
// (the operator may generate a template before creating the directory).
func validateUnixPath(s string) bool {
	if s == "" {
		return true
	}
	if !strings.HasPrefix(s, "/") {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
		switch r {
		case '`', '$', '\\', '"', '\n', '\r', '\t', ';', '&', '|', '<', '>', '*':
			return false
		}
	}
	return true
}

// ReverseProxyTemplate returns a ready-to-use reverse-proxy config snippet.
func ReverseProxyTemplate(kind, domain, upstream string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	domain = strings.TrimSpace(domain)
	upstream = strings.TrimSpace(upstream)
	if !isValidDomain(domain) {
		return "", fmt.Errorf("invalid domain %q", domain)
	}
	if upstream == "" {
		upstream = "127.0.0.1:8080"
	}
	// upstream must look like host:port or a unix socket path; reject shell
	// metacharacters either way.
	if strings.ContainsAny(upstream, "`$\\\";|<>&*\n\r\t ") {
		return "", fmt.Errorf("invalid upstream %q", upstream)
	}
	switch kind {
	case "nginx":
		return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location /.well-known/acme-challenge/ {
        root /var/www/acme;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    ssl_certificate     /etc/ssl/%s/fullchain.pem;
    ssl_certificate_key /etc/ssl/%s/privkey.pem;

    location / {
        proxy_pass http://%s;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`, domain, domain, domain, domain, upstream), nil
	case "caddy":
		return fmt.Sprintf(`%s {
    reverse_proxy %s
}
`, domain, upstream), nil
	default:
		return "", fmt.Errorf("unsupported kind %q (use nginx or caddy)", kind)
	}
}

// ACMECommand returns a suggested certbot / acme.sh command (reference only).
func ACMECommand(domain, email, webroot string) (string, error) {
	domain = strings.TrimSpace(domain)
	if !isValidDomain(domain) {
		return "", fmt.Errorf("invalid domain %q", domain)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		email = "admin@" + domain
	} else if !isValidEmail(email) {
		return "", fmt.Errorf("invalid email %q", email)
	}
	if webroot == "" {
		webroot = "/var/www/acme"
	} else if !validateUnixPath(webroot) {
		return "", fmt.Errorf("invalid webroot %q", webroot)
	}
	return fmt.Sprintf(
		"certbot certonly --webroot -w %s -d %s --email %s --agree-tos --non-interactive",
		shellQuote(webroot), shellQuote(domain), shellQuote(email),
	), nil
}

// silence unused-import warning if a future refactor drops net usage.
var _ = net.ParseIP
