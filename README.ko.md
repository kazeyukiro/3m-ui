# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · **한국어** · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)


**Mihomo 서버 웹 관리 패널**

> **안정판 우선:** 기본 설치/업데이트는 정식 Release만 사용합니다. 사전 배포는 명시적 선택. [설치 문서](https://3m-ui.top/docs/install.html).

가벼운 셀프호스팅 패널. Linux에서 [Mihomo](https://github.com/MetaCubeX/mihomo) 리스너·사용자·구독·상태를 관리합니다.

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> 문서: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## 기능

| 구분 | 기능 |
|------|------|
| **노드** | Protocol-registry listeners (VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, …); REALITY target scan; TLS self-signed certs; transport field validation |
| **사용자** | Bind nodes, traffic limits, expiry, IP/HWID limits, batch ops, subscription tokens |
| **구독** | UA routing for Clash/Mihomo YAML, v2ray Base64, sing-box JSON; optional `?target=`; HTML info page |
| **설정** | Generate → validate → apply; rollback previous `config.yaml` on failure |
| **운영** | Core start/stop/update, logs, dashboard metrics, Geo files, panel SSL/ACME, backup/restore |
| **Telegram** | Alerts and bot commands (token + chat IDs) |
| **클러스터** | Register remote panels, health checks, node mirror sync, merged subscriptions |

---

## 설치

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

설치 후 업데이트:

```bash
sudo 3m-ui update --stable
# sudo 3m-ui update v1.0.2
```

기본 패널: `http://SERVER_IP:8080/` — 사용자 `admin`, 일회성 무작위 비밀번호(**첫 로그인 시 변경**).

---

## 소스 빌드

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

다중 아키텍처 정적 바이너리: [Releases](https://github.com/kazeyukiro/3m-ui/releases).

---

## 감사

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX inspiration
- Gin, GORM, React, Ant Design, Zustand, golang-jwt/jwt
- Go, Node.js, and the open-source community

기여자: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx)

---

## 라이선스

[Eclipse Public License 2.0](./LICENSE)

서드파티: [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md).
