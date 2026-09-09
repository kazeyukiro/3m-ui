# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](./README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · **繁體中文** · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)


**Mihomo 服務端 Web 管理面板**

> **穩定版優先：** 預設安裝與更新僅使用正式 Release。預發布需主動選擇。完整安裝包及 Docker 映像包含固定版本 Mihomo；詳見 [安裝文件](https://3m-ui.top/docs/install.html)。

輕量、自託管，用於在 Linux 上管理 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener、使用者、訂閱與執行狀態。

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> 文件: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## 功能

| 類別 | 能力 |
|------|------|
| **節點** | 協定登錄表驅動的 Listener（VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC 等）；REALITY 目標掃描、TLS 自簽、傳輸層欄位校驗 |
| **使用者** | 綁定節點、流量限額、到期、IP/HWID 限制、批次操作、訂閱 Token |
| **訂閱** | UA 識別 Clash/Mihomo YAML、v2ray Base64、sing-box JSON；可選 `?target=`；HTML 訂閱頁 |
| **設定** | 產生 → 校驗 → 套用；失敗回滾上一份 `config.yaml` |
| **維運** | 核心啟停/更新、日誌、儀表板、Geo、面板 SSL/ACME、備份還原 |
| **Telegram** | 告警与管理命令（需 Token + Chat ID） |
| **多機** | 登記遠端面板、健康檢查、節點鏡像同步、合併訂閱 |

---

## 安裝

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

安裝後更新：

```bash
sudo 3m-ui update --stable
# sudo 3m-ui update v1.0.2
```

預設面板：`http://伺服器IP:8080/` — 使用者 `admin`，安裝時列印一次性隨機密碼（**首次登入請立即修改**）。

---

## 原始碼編譯

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

多架構靜態 Linux 二進位見 [Releases](https://github.com/kazeyukiro/3m-ui/releases)。

---

## 致謝

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心引擎与 Listener 模型
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 示例
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — 面板交互参考
- Gin、GORM、React、Ant Design、Zustand、golang-jwt/jwt
- Go、Node.js 与开源社区

貢獻者: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx)

---

## 授權

[Eclipse Public License 2.0](./LICENSE)

第三方元件見 [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md)。
