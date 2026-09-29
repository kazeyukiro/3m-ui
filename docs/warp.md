# Cloudflare WARP (m-ui style)

3m-ui can **register and persist** a Cloudflare WARP account, inject a Mihomo outbound named **`WARP`**, and route selected domains through it on the **server**.

## Settings

**Settings → Network → Cloudflare WARP**

1. Choose WireGuard (recommended for server egress) or MASQUE  
2. **Register / save WARP** — creates a device at Cloudflare, stores the account in panel settings, and builds outbound `WARP`  
3. Optional: view YAML; **Delete WARP account** removes the stored device  

API:

- `GET /api/v1/system/warp` — account status (no secrets)  
- `POST /api/v1/system/warp` — register + save account  
- `DELETE /api/v1/system/warp` — delete account  
- `POST /api/v1/system/templates/warp/register` — still available; **also saves** the account unless `?nosave=1`

## Server egress (Routing)

**Routing → Server egress**

1. **WARP domains**: one domain per line (e.g. `openai.com`) or `GEOSITE:openai`  
2. Edit extra rules if needed (default `MATCH,DIRECT`)  
3. **Save → Generate & apply**  

The generator:

- Injects the **WARP** WireGuard outbound from the saved account  
- Prepends `DOMAIN-SUFFIX,<domain>,WARP` (or GEOSITE) for each WARP domain  
- Fails apply if WARP domains are set but no account exists  

## Limits

WARP does **not** guarantee Netflix / AI unlock. Domain list is operator-controlled (same idea as m-ui `WarpDomains`).
