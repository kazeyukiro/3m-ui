# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · **Bahasa Indonesia** · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)


**Panel web manajemen server Mihomo**

> **Stabil dulu:** instalasi dan pembaruan default memakai rilis formal.

Ringan dan self-hosted. Kelola listener, pengguna, langganan, dan status [Mihomo](https://github.com/MetaCubeX/mihomo) di Linux.

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> Dokumentasi: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## Fitur

| Area | Kemampuan |
|------|------|
| **Node** | Protocol-registry listeners (VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, …); REALITY target scan; TLS self-signed certs; transport field validation |
| **Pengguna** | Bind nodes, traffic limits, expiry, IP/HWID limits, batch ops, subscription tokens |
| **Langganan** | UA routing for Clash/Mihomo YAML, v2ray Base64, sing-box JSON; optional `?target=`; HTML info page |
| **Konfigurasi** | Generate → validate → apply; rollback previous `config.yaml` on failure |
| **Operasi** | Core start/stop/update, logs, dashboard metrics, Geo files, panel SSL/ACME, backup/restore |
| **Telegram** | Alerts and bot commands (token + chat IDs) |
| **Klaster** | Register remote panels, health checks, node mirror sync, merged subscriptions |

---

## Instalasi

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Setelah instal, perbarui dengan:

```bash
sudo 3m-ui update
# sudo 3m-ui update v1.0.2
```

Panel default: `http://SERVER_IP:8080/` — user `admin`, kata sandi acak sekali pakai (**ubah saat login pertama**).

---

## Build dari sumber

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Biner statis: [Releases](https://github.com/kazeyukiro/3m-ui/releases).

---

## Ucapan terima kasih

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX inspiration
- Gin, GORM, React, Ant Design, Zustand, golang-jwt/jwt
- Go, Node.js, and the open-source community

Kontributor: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx)

---

## Lisensi

[Eclipse Public License 2.0](./LICENSE)

Pemberitahuan pihak ketiga: [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md).
