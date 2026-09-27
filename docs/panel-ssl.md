# Panel SSL (HTTPS / ACME)

Panel TLS is configured under **Settings → Certificates / SSL** (API: `GET/PUT /api/v1/system/ssl`). Changes apply after **restarting the panel** (`systemctl restart 3m-ui` or the in-panel restart action).

## Modes

| Mode | When | Notes |
|------|------|--------|
| **HTTP-01** (default) | Single hostname, port **80** reachable from the internet | `golang.org/x/crypto/acme/autocert` |
| **DNS-01** | Wildcard `*.example.com`, or no usable port 80 | Cloudflare API token; via `acmez` |
| **IP certificate** | Domain field is a public IP | Short-lived Let’s Encrypt profile; HTTP-01 / TLS-ALPN-01 |
| **Manual** | `cert_file` + `key_file` set | Paths must be under allowlisted dirs (e.g. `/etc/letsencrypt/`, `/var/lib/3m-ui/`) |

## Wildcard (`*.example.com`)

1. Set **Domain** to `*.example.com` (or a normal hostname with **Challenge = DNS-01**).
2. Set **ACME challenge** to **DNS-01**.
3. **DNS provider**: Cloudflare (currently the only built-in provider).
4. **DNS API token**: Cloudflare token with **Zone → DNS → Edit** on that zone. Leave the field blank on later saves to keep the stored token.
5. **DNS zone** (optional): e.g. `example.com` if auto-detect fails.
6. Save and **restart** the panel. Issuance runs at startup (may take tens of seconds).

The certificate includes both `*.example.com` and the apex `example.com`.

**Not supported as automatic ACME without DNS-01:** filling only `*.example.com` with HTTP-01.

### Cloudflare token

- Permission: `Zone` → `DNS` → **Edit**
- Resource: the zone that hosts the domain

## API fields (`PUT /system/ssl`)

| Field | Description |
|-------|-------------|
| `enabled` | Enable panel HTTPS |
| `domain` | Hostname, `*.example.com`, or public IP |
| `email` | ACME account contact |
| `challenge` | empty / `http-01` (default) or `dns-01` |
| `dns_provider` | `cloudflare` |
| `dns_token` | Provider API token (not returned on `GET`; empty on PUT keeps previous) |
| `dns_zone` | Optional zone name |
| `cache_dir` | ACME cache (default under data dir, e.g. `/var/lib/3m-ui/acme`) |
| `cert_file` / `key_file` | Manual PEM paths |
| `listen_http` / `listen_tls` | e.g. `:80` / `:443` |

`GET /system/ssl/status` includes `mode` (`letsencrypt`, `letsencrypt-dns01`, `letsencrypt-ip`, `manual`, …), `is_wildcard`, `has_dns_token`, etc.

## Misconfiguration / recovery

```bash
3m-ui reset-config --panel --yes   # or use the SSH menu reset option
systemctl restart 3m-ui
```

You do **not** need a full reinstall to undo a bad SSL setting.

## Related

- [Batch apply certificate to nodes](./batch-certificate.md)
- Nodes can reuse panel manual cert paths via **Apply cert** → `from_panel_ssl` when panel SSL uses file paths (ACME cache formats differ; prefer manual PEMs or Let’s Encrypt live paths for listeners).
