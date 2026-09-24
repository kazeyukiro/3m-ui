# Routing (client subscription)

The **Routing** page edits **client subscription** layout only: Mihomo/Clash `proxy-groups` and `rules` stored in the `visual-config` fragment.

- Community templates (YiXuanZX / echs-top / AIsouler) fill groups + rules for the **client** YAML. Switching a template **replaces** rules and groups (does not only append).
- After apply/save, update the subscription in the client (Mihomo/Clash YAML target).
- The **panel Mihomo process** always uses `MATCH,DIRECT` (inbound panel). Visual rules/groups/proxies are **not** merged into the serving `config.yaml`.

Cloudflare WARP registration is under **Settings**, not Routing. See [WARP](warp.md).

## Config engine

**Generate & apply** rebuilds listener config for the server core. It does not apply client community rules to the host process.
