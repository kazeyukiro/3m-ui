# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · **Deutsch** · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)


**Web-Verwaltungspanel für Mihomo-Server**

> **Stable first:** Installation und Update nutzen standardmäßig formale Releases.

Leichtgewichtig und selbst gehostet. Verwalten Sie [Mihomo](https://github.com/MetaCubeX/mihomo)-Listener, Benutzer, Abonnements und Laufzeitstatus unter Linux.

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> Dokumentation: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## Funktionen

| Bereich | Fähigkeiten |
|------|------|
| **Knoten** | Protocol-registry listeners (VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, …); REALITY target scan; TLS self-signed certs; transport field validation |
| **Benutzer** | Bind nodes, traffic limits, expiry, IP/HWID limits, batch ops, subscription tokens |
| **Abonnements** | UA routing for Clash/Mihomo YAML, v2ray Base64, sing-box JSON; optional `?target=`; HTML info page |
| **Konfiguration** | Generate → validate → apply; rollback previous `config.yaml` on failure |
| **Betrieb** | Core start/stop/update, logs, dashboard metrics, Geo files, panel SSL/ACME, backup/restore |
| **Telegram** | Alerts and bot commands (token + chat IDs) |
| **Cluster** | Register remote panels, health checks, node mirror sync, merged subscriptions |

---

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Nach der Installation aktualisieren mit:

```bash
sudo 3m-ui update
# sudo 3m-ui update v1.0.2
```

Standardpanel: `http://SERVER_IP:8080/` — Benutzer `admin`, einmaliges Zufallskennwort (**beim ersten Login ändern**).

---

## Aus Quellcode bauen

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Statische Binaries: [Releases](https://github.com/kazeyukiro/3m-ui/releases).

---

## Danksagungen

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX inspiration
- Gin, GORM, React, Ant Design, Zustand, golang-jwt/jwt
- Go, Node.js, and the open-source community

Mitwirkende: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx)

---

## Lizenz

[Eclipse Public License 2.0](./LICENSE)

Drittanbieter: [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md).
