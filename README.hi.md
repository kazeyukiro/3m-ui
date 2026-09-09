# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · **हिन्दी** · [Polski](./README.pl.md) · [Українська](./README.uk.md)


**Mihomo सर्वर के लिए वेब प्रबंधन पैनल**

> **पहले स्थिर रिलीज़:** इंस्टॉल/अपडेट डिफ़ॉल्ट रूप से औपचारिक रिलीज़ उपयोग करते हैं।

हल्का, स्व-होस्टेड। Linux पर [Mihomo](https://github.com/MetaCubeX/mihomo) listeners, उपयोगकर्ता, सब्सक्रिप्शन और स्थिति प्रबंधित करें।

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> दस्तावेज़: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## विशेषताएँ

| क्षेत्र | क्षमताएँ |
|------|------|
| **नोड्स** | Protocol-registry listeners (VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, …); REALITY target scan; TLS self-signed certs; transport field validation |
| **उपयोगकर्ता** | Bind nodes, traffic limits, expiry, IP/HWID limits, batch ops, subscription tokens |
| **सब्सक्रिप्शन** | UA routing for Clash/Mihomo YAML, v2ray Base64, sing-box JSON; optional `?target=`; HTML info page |
| **कॉन्फ़िग** | Generate → validate → apply; rollback previous `config.yaml` on failure |
| **ऑप्स** | Core start/stop/update, logs, dashboard metrics, Geo files, panel SSL/ACME, backup/restore |
| **Telegram** | Alerts and bot commands (token + chat IDs) |
| **क्लस्टर** | Register remote panels, health checks, node mirror sync, merged subscriptions |

---

## इंस्टॉल

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

इंस्टॉल के बाद अपडेट:

```bash
sudo 3m-ui update --stable
# sudo 3m-ui update v1.0.2
```

डिफ़ॉल्ट पैनल: `http://SERVER_IP:8080/` — उपयोगकर्ता `admin`, एक-बार यादृच्छिक पासवर्ड (**पहली लॉगिन पर बदलें**).

---

## स्रोत से बिल्ड

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

स्टैटिक बाइनरी: [Releases](https://github.com/kazeyukiro/3m-ui/releases).

---

## आभार

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX inspiration
- Gin, GORM, React, Ant Design, Zustand, golang-jwt/jwt
- Go, Node.js, and the open-source community

योगदानकर्ता: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx)

---

## लाइसेंस

[Eclipse Public License 2.0](./LICENSE)

तृतीय-पक्ष सूचनाएँ: [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md).
