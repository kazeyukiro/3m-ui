package system

import (
	"fmt"
	"strings"
)

// ReverseProxyTemplate returns a ready-to-use reverse-proxy config snippet.
func ReverseProxyTemplate(kind, domain, upstream string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	domain = strings.TrimSpace(domain)
	upstream = strings.TrimSpace(upstream)
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if upstream == "" {
		upstream = "127.0.0.1:8080"
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
func ACMECommand(domain, email, webroot string) string {
	domain = strings.TrimSpace(domain)
	email = strings.TrimSpace(email)
	if email == "" {
		email = "admin@" + domain
	}
	if webroot == "" {
		webroot = "/var/www/acme"
	}
	return fmt.Sprintf(
		"certbot certonly --webroot -w %s -d %s --email %s --agree-tos --non-interactive",
		webroot, domain, email,
	)
}
