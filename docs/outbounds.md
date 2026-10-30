# Outbound management

**入站管理** manages Mihomo listeners. **出站管理** manages the outbound
`proxies:` used by the serving Mihomo process. Client subscription proxies and
routing remain separate.

Create, edit, clone, delete, search, refresh, and individually or bulk
enable/disable outbounds from the new page. Basic connection fields appear
first; optional protocol, transport, TLS, dial, multiplexing, and WireGuard
parameters are grouped below. Optional parameters use Mihomo's field names.
The field catalogue follows the bundled Mihomo v1.19.33 option structs.
Optional booleans offer enabled, disabled, and Mihomo default states; clearing
an optional field restores the core default.

Supported choices match 入站管理 plus WireGuard: Shadowsocks, Snell, VMess,
VLESS, Trojan, Hysteria2, TUIC v4/v5, ShadowQUIC, AnyTLS, Mieru, Sudoku,
TrustTunnel, HTTP, and SOCKS. The UI maps Shadowsocks to `ss`, SOCKS to
`socks5`, and both TUIC versions to `tuic` with the appropriate credentials.
WireGuard supports one root peer or a structured peer list.

## Save and apply

1. Save an outbound. This persists it in the panel database.
2. Choose an enabled outbound in **路由规则 → 服务端出站** and save routing.
3. Click **应用** to generate, validate, and reload Mihomo using the existing
   configuration workflow. Saving alone does not change the running core.

Renaming updates server rule targets, group members, rule-provider download
proxies, and other outbounds' `dialer-proxy` references atomically. Matching
domains, regex payloads, and client subscription configuration are preserved.
Deletion and disabling are blocked while references remain; the error lists
their locations. Remove those references first. Disabled definitions remain
editable in the database and are omitted from generated YAML.

## WARP

A saved Settings WARP account appears automatically as one enabled, read-only
`WARP` outbound. Registration and re-registration retain Settings ownership;
the outbound list does not expose its private key or Cloudflare access token.
Use the Settings link to manage the account. Account deletion is blocked while
rules, groups, providers, dialer references, WARP domains, or global WARP still
use it. Apply again after deleting the account.

## Configuration ownership and storage

Outbound definitions remain in `panel_settings[server-routing].proxies`.
`disabledOutboundNames` is panel-only metadata beside them. No database table
or schema migration is added. The generator writes the enabled outbound list,
including an empty list when the last outbound is removed.

Existing non-empty `proxies:` sections in enabled custom fragments produce an
actionable generation error identifying the fragment. Move those definitions
into 出站管理 and remove the fragment's `proxies:` section before applying.
Existing panel-owned server-routing definitions remain available. Client
`visual-config` proxies continue to belong to subscriptions.

## API

All endpoints require panel authentication and use `/api/v1`:

| Method | Endpoint | Behavior |
| --- | --- | --- |
| GET | `/config/server-outbounds` | List `{proxy, enabled, managed}` entries, including configured WARP |
| POST | `/config/server-outbounds` | Create a `{proxy, enabled}` entry |
| PUT | `/config/server-outbounds?name=existing` | Edit or rename an entry |
| DELETE | `/config/server-outbounds?name=existing` | Delete an unreferenced entry |
| POST | `/config/server-outbounds/enabled` | Atomically set `{names: [...], enabled: boolean}` |
| PATCH | `/config/server-routing` | Update routing fields while preserving outbound definitions and disabled state |

Names in query parameters must be URL encoded. Invalid definitions return 400,
missing entries 404, and dependency conflicts 409. The routing PATCH accepts
`rules`, `proxyGroups`, `warpDomains`, `warpGlobal`, and `ruleProviders`;
omitted fields remain unchanged. The existing full-replacement routing PUT
remains available.

For the real-core fixture check, set `MIHOMO_TEST_BINARY` to the bundled
Mihomo executable and run `go test ./internal/mihomo/config` from `backend/`.
