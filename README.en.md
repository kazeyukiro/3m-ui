# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · **English** · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo server web management panel**

> **Stable first:** install and update use formal releases by default. Pre-releases require an explicit choice. Full bundles and Docker images ship a pinned Mihomo core — see [installation docs](https://3m-ui.top/docs/install.html).

Lightweight and self-hosted. Manage [Mihomo](https://github.com/MetaCubeX/mihomo) listeners, users, subscriptions, and runtime status on Linux.

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> Documentation: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## Features

| Area | Capabilities |
|------|------|
| **Nodes** | Protocol-registry listeners (VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, …); REALITY; TLS self-signed; batch certificate; **traffic multiplier**; transport validation |
| **Users** | Bind nodes, traffic/expiry, **per-node usage** (billed = raw × node multiplier), IP/sub-pull limits, start-on-first-use, cycle renew/reset, groups/tags, external sub merge, batch ops, subscription tokens |
| **Subscriptions** | UA routing for Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` always Base64), sing-box JSON; optional `?target=`; HTML info page; TUIC/HY2 share TLS params |
| **Config** | Generate → validate → apply; rollback previous `config.yaml` on failure |
| **Telegram** | Alerts and bot commands (token + chat IDs) |
| **Cluster** | Register remote panels, health checks, node mirror sync, merged subscriptions; **push local nodes by name** (created disabled on remote) |
| **Ops** | Core lifecycle, Geo, panel SSL/ACME, backup, **Cloudflare WARP** one-click register (WireGuard/MASQUE YAML) |

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

After install, update with:

```bash
sudo 3m-ui update
# sudo 3m-ui update v1.0.2
```

Default panel: `http://SERVER_IP:8080/` — user `admin`, one-time random password printed at install (**change on first login**).

---

### Install with an AI assistant prompt

Copy the prompt into ChatGPT / Claude / Cursor (and state your OS, e.g. Ubuntu 22.04, and whether you have root SSH). The assistant should only use the official installer — not rewrite URLs or invent third-party commands. The full prompt and usage notes live in [the AI install prompt doc](docs/ai-install-prompt.md).

Skip manual copy-paste — load the plain-text prompt straight to your clipboard with `curl`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

The prompt only ever runs the official one-liner below; all downloads verify `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

For the optional pre-release / test channel, see [installation docs](docs/installation.md#ai-提示词安装).

## Build from source

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Static Linux binaries for multiple architectures are on [Releases](https://github.com/kazeyukiro/3m-ui/releases).

---

## Acknowledgements

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX inspiration
- [Gin](https://github.com/gin-gonic/gin), [GORM](https://github.com/go-gorm/gorm), [React](https://github.com/facebook/react), [Ant Design](https://github.com/ant-design/ant-design)
- [Lucide](https://github.com/lucide-icons/lucide) — icons (`lucide-react`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo routing config / lite rule ideas
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo rules and proxy-group schemes
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — Rule sets and mihomo routing templates
- [Zustand](https://github.com/pmndrs/zustand), [golang-jwt/jwt](https://github.com/golang-jwt/jwt)

### Extra docs (limits / certs / subscriptions)

- [User limits: IP · sub pulls](docs/users-limits.md)
- [Per-node traffic & multiplier](docs/node-traffic.md)
- [Batch node certificates](docs/batch-certificate.md)
- [Subscription formats](docs/subscription-formats.md)
- [Cluster / push nodes](docs/cluster.md)
- [Routing & groups](docs/routing.md) · [Cloudflare WARP](docs/warp.md)

- Go, Node.js, and the open-source community

Contributors: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## License

[Eclipse Public License 2.0](./LICENSE)

Third-party notices: [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md).
