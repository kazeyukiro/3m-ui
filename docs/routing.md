# Routing & proxy-groups

Panel page **Routing** edits Mihomo **outbound** policy stored in the `visual-config` fragment (not inbound Listeners).

## Defaults

Fresh install uses:

```text
MATCH,DIRECT
```

That is intentional for a **server inbound** panel: client traffic that reaches this host leaves via DIRECT unless you add rules.

## Structured rules (UI)

Each rule has:

| Field | Meaning |
|-------|---------|
| Type | e.g. `DOMAIN`, `DOMAIN-SUFFIX`, `GEOIP`, `GEOSITE`, `IP-CIDR`, `MATCH`, … |
| Payload | Match value (empty for `MATCH`) |
| Target | `DIRECT`, `REJECT`, a **proxy-group** name, or an outbound **proxy** name |
| no-resolve | Optional on `IP-CIDR` / `IP-CIDR6` / `GEOIP` |

Rules can be reordered. Advanced lines that the UI cannot parse are kept as **raw** and saved unchanged.

**Save** writes the database. Rules take effect only after **Generate & apply** (same path as node config).

### Validation

- Type, target required; payload required except `MATCH`
- Warning if the last rule is not `MATCH`

### Templates

| Template | Result |
|----------|--------|
| DIRECT only | `MATCH,DIRECT` |
| CN direct | `GEOIP,CN,DIRECT` + `MATCH,<first group or PROXY>` |
| Sample ads | Several `DOMAIN-*` → `REJECT` + `MATCH,DIRECT` |
| Via group | `MATCH,<first group or PROXY>` |

### WARP inject

**Routing → WARP** calls `POST /api/v1/config/routing/inject-warp`:

1. Registers Cloudflare WARP
2. Merges the outbound into `visual-config` proxies
3. Optional rule update: none / final `MATCH,<name>` / `GEOIP,CN,DIRECT` + `MATCH,<name>`

Then generate & apply. Requires the panel host to reach Cloudflare.

## Proxy-groups

Simple groups: `select` / `url-test` / `fallback` / `load-balance`, member list, optional URL/interval for url-test.

## APIs

```http
GET/PUT /api/v1/config/rules
GET/PUT /api/v1/config/groups
POST    /api/v1/config/routing/inject-warp
```

```json
{ "mode": "wireguard", "rule_mode": "match", "name": "WARP-OUT" }
```

`rule_mode`: `none` | `match` | `cn_direct`.

Also related: `GET/POST /api/v1/config/proxies`, visual config under **Config**.

## Related

- [Installation](installation.md) — frontend stack
- [Cluster](cluster.md) — multi-panel (separate from Mihomo rules)


## Community rule templates

Routing → **Templates** includes adaptations of public Mihomo rule projects (for server visual-config; not a full client overwrite script):

| Template | Upstream | Intent |
|----------|----------|--------|
| YiXuanZX/rules | [YiXuanZX/rules](https://github.com/YiXuanZX/rules) | Private/CN direct, Telegram + GFW → PROXY |
| echs-top/proxy | [echs-top/proxy](https://github.com/echs-top/proxy) | Light ad reject, CN direct, GFW → PROXY |
| AIsouler/MyClash | [AIsouler/MyClash](https://github.com/AIsouler/MyClash) | CN direct, Google/Telegram/OpenAI/GFW → PROXY |

These use **GEOSITE/GEOIP** against MetaCubeX geodata (update under Settings → Geo). Full upstream configs with remote `rule-providers` remain optional for advanced custom YAML fragments.
