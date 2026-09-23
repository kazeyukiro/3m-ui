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
| YiXuanZX/rules | [YiXuanZX/rules](https://github.com/YiXuanZX/rules) | Groups: 香港/新加坡/日本/美国/其他 + **代理 / AI / TG**. Rules: CN direct, OpenAI→AI, Telegram→TG, GFW→代理 |
| echs-top/proxy | [echs-top/proxy](https://github.com/echs-top/proxy) | Groups: **代理连接 / TELEGRAM / 国外AI / GOOGLE / 海外媒体 / …**. Ads REJECT + CN direct + category rules |
| AIsouler/MyClash | [AIsouler/MyClash](https://github.com/AIsouler/MyClash) | Groups: **直连 / AdBlock / Google / AI / Telegram / Steam / 默认代理 / 漏网之鱼**. Lite GEOSITE map |

Applying a community template **writes matching proxy-groups** (merged by name) and rules. Leaf members use existing visual outbounds if any, else `DIRECT` — put real nodes/WARP into the region or policy groups afterward.

These use **GEOSITE/GEOIP** against MetaCubeX geodata (Settings → Geo). Full upstream `rule-providers` / region filters remain optional via custom YAML fragments.
