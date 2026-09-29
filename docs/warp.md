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

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| `UNIQUE constraint failed: panel_settings.key` | Soft-deleted row still held the unique key (fixed: Unscoped upsert + hard delete) |
| Domains set but traffic still direct | Did not **Generate & apply** after save; or no WARP account |
| Account shows configured, Apply fails | ProxyMap / core validation — check logs; ensure private key present |
| MASQUE selected but config is WireGuard | **By design**: server egress always injects WireGuard `WARP`; MASQUE is preview YAML only |
| After delete, still seeing WARP in core | Re-apply config so generator omits the outbound |

Flow: **Settings → Register WARP** → **Routing → Server egress → WARP domains** → **Save → Generate & apply**.
