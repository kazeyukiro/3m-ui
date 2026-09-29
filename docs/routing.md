# Routing

The **Routing** page has two scopes:

| Tab | Stored as | Affects |
|-----|-----------|---------|
| **Client subscription** | `visual-config` | Mihomo/Clash **subscription** YAML (`proxy-groups` / `rules`) only |
| **Server egress** | `server-routing` | **Panel Mihomo process** after traffic hits listeners (like 3x-ui Xray outbounds + routing) |

## Client subscription

- Community templates (YiXuanZX / echs-top / AIsouler) fill groups + rules for the **client** YAML. Switching a template **replaces** rules and groups (does not only append).
- After save, **update the subscription in the client** (Mihomo/Clash YAML target).
- These rules are **not** merged into the serving `config.yaml` (by design).

## Server egress

Same idea as 3x-ui server-side routing: decide how traffic **leaves the VPS** after a user connects to a node.

- Default: `MATCH,DIRECT` (historical behaviour — all egress direct from the host).
- You can add Mihomo rules (e.g. `GEOIP,private,DIRECT`, domain rules) and optional `proxies` / `proxy-groups` (e.g. WARP outbound pasted from Settings).
- **Save → Generate & apply** so the core reloads with the new rules.
- API: `GET/PUT /api/v1/config/server-routing` body `{ "proxies": [], "proxyGroups": [], "rules": ["MATCH,DIRECT"] }`.
- Last rule should include a `MATCH,...` line (panel appends `MATCH,DIRECT` if missing).

GEOIP/GEOSITE rules need Geo data on the host (Settings → update Geo files).

## Config engine

**Generate & apply** rebuilds listeners + **server egress** into the serving config. Client community rules stay subscription-only.

Cloudflare WARP registration is under **Settings**. See [WARP](warp.md) for registering an outbound and using it under **Server egress**.

## 中文

| 标签 | 作用 |
|------|------|
| **客户端订阅** | 只进 Clash/Mihomo 订阅，改完请在客户端更新订阅 |
| **服务端出站** | 写入面板 Mihomo（用户流量进节点之后的出口），类似 3x-ui 服务端路由；默认 `MATCH,DIRECT`；保存后请「生成并应用」 |

WARP 在 **设置** 里一键注册，把 YAML 作为出站合并进服务端规则即可（不会自动注入）。
