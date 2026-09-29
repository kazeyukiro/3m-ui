# Cloudflare WARP

3m-ui can **register and persist** a Cloudflare WARP account, inject a Mihomo outbound named **`WARP`**, and route selected domains through it on the **server**.

## Settings

**Settings → Network → Cloudflare WARP**

1. Choose WireGuard (recommended for server egress) or MASQUE  
2. **Register / save WARP** — creates a device at Cloudflare, stores the account, builds outbound `WARP`  
3. Optional YAML preview; **Delete WARP account** **hard-deletes** the row (avoids SQLite `UNIQUE(panel_settings.key)` errors after soft-delete)

API:

- `GET /api/v1/system/warp` — status (no secrets)  
- `POST /api/v1/system/warp` — register + save  
- `DELETE /api/v1/system/warp` — delete account  
- `POST /api/v1/system/templates/warp/register` — legacy; also saves unless `?nosave=1`

## Server egress

**Routing → Server egress**

1. **WARP domains**: one domain per line (or `GEOSITE:name`)  
2. Extra rules if needed (default `MATCH,DIRECT`)  
3. **Save → Generate & apply** (required for domains to appear in serving `config.yaml`)

The generator injects WireGuard outbound `WARP` (Cloudflare peer public key + `allowed-ips`) and prepends `DOMAIN-SUFFIX,…,WARP` rules.

If WARP domains are set but no account is saved, apply fails with a clear error.

## Limits

WARP does **not** guarantee streaming / AI unlock. Domain list is operator-controlled.
