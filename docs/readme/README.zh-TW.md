# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · **繁體中文** · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo 服務端 Web 管理面板**

> **穩定版優先：** 預設安裝與更新僅使用正式 Release。預發布需要主動選擇。完整安裝包及 Docker 映像包含固定版本的 Mihomo；詳見 [安裝、升級與還原](https://3m-ui.top/guide/install)。

輕量、自託管，用於在 Linux 上管理 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener、使用者、訂閱與執行狀態。

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> 文件：[https://3m-ui.top/docs/](https://3m-ui.top/docs/) · 官網：[https://3m-ui.top/](https://3m-ui.top/)

---

## 特性

| 類別 | 能力 |
|------|------|
| **節點** | 協定註冊表驅動的 Listener：VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC 等；Reality、TLS 自簽、一鍵批量憑證、**流量倍率**、傳輸層欄位互斥校驗 |
| **使用者** | **一鍵建立**（隨機使用者名稱/密碼/UUID，可選綁定全部節點）、綁定節點、流量限額、**按節點分別計量**（計費 = 實際 × 節點倍率）、到期、IP/訂閱拉取限制、首次使用起算、週期續期/重置、分組標籤、外部訂閱合併、批量操作、訂閱 Token |
| **訂閱** | UA 自動辨識 Clash/Mihomo YAML、v2ray Base64（`?target=v2ray` 始終 Base64）、sing-box JSON；可選 `?target=`；HTML 訂閱頁；TUIC/HY2 分享鏈含 SNI/`allow_insecure` 等用戶端參數 |
| **設定** | 產生 → 校驗 → 套用 分離；失敗回滾上一份 `config.yaml` |
| **Telegram** | 告警與管理指令（需 Token + Chat ID） |
| **多機** | 登記遠端面板、健康檢查、同步節點鏡像、合併訂閱；**按名稱選擇本機節點推送**到遠端（預設停用，可改名/連接埠） |
| **安全** | JWT、首登改密、TOTP 兩步驟驗證、可選 **GitHub OAuth**、web_path 路徑前綴、憑據加密、CORS |
| **維運** | 核心啟停/更新、Geo、面板 SSL/ACME（含 DNS-01 通配符）、備份、**Cloudflare WARP** 一鍵註冊（WireGuard/MASQUE YAML） |

### 補充說明（使用者限制 / 憑證 / 訂閱）

- [使用者限制：IP · 訂閱拉取](https://3m-ui.top/guide/users-limits)
- [按節點流量與倍率](https://3m-ui.top/guide/node-traffic)
- [面板 SSL / ACME（HTTP-01、DNS-01 通配符）](https://3m-ui.top/guide/ssl-cert)
- [批量套用節點憑證](https://3m-ui.top/guide/batch-certificate)
- [訂閱格式與 target](https://3m-ui.top/guide/subscription-formats)
- [多機節點 / 推送](https://3m-ui.top/guide/cluster)
- [路由（用戶端訂閱 + 服務端出站）](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

前端：**React + Ant Design + Lucide Icons**（`frontend/`），建置後嵌入單一 Go 二進位。

發布產物：**純靜態連結**（`CGO_ENABLED=0` + modernc SQLite），無需系統 `libsqlite3`，相容 glibc / musl。

完整安裝包 / Docker：`linux/amd64` · `arm64`。進階獨立面板二進位另外支援 `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`，需要自行準備相容核心。

---

## 快速安裝

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

完整安裝支援 Linux amd64 / arm64，自動安裝面板、配套 Mihomo 與系統服務。首次設定與管理員密碼由程式統一產生；設定已存在時保持原值。其他架構可使用獨立二進位與自行準備的核心，見 [詳細安裝說明](https://3m-ui.top/guide/install)。

安裝完成後：

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

更新：

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

卸載：

```bash
sudo 3m-ui uninstall
```

### AI 提示詞安裝

把提示詞複製到 ChatGPT / Claude / Cursor 等助手，並說明你的系統（如 Ubuntu 22.04）與是否已有 root SSH。助手應只協助執行官方腳本，不要改寫成不明來源指令。完整提示詞及使用說明見 [AI 提示詞安裝文件](../ai-install-prompt.md)。

無需手動複製大段文字，用 `curl` 直接把純文字提示詞載入到剪貼簿再貼上：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | wl-copy       # Linux Wayland
```

提示詞只引導使用下方官方一鍵腳本，所有下載均校驗 `SHA256SUMS`：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

測試 / 預發布通道（可選，非預設）見 [安裝文件](../installation.md#ai-提示词安装)。

---

## Docker 安裝

Linux VPS 下載發布版本中的 `docker-compose.yml`，在同一目錄執行：

```bash
docker compose up -d
docker compose logs 3m-ui
```

映像為 `ghcr.io/kazeyukiro/3m-ui:latest`，包含面板及固定版本的 Mihomo；`latest` 只跟隨穩定版。設定、隨機金鑰與管理員會自動初始化，首次密碼只在首次啟動輸出一次。預設 host 網路，新增節點後放行實際使用的 TCP/UDP 連接埠。另提供 `docker-compose.bridge.yml`，以及版本固定、憑證設定與持久化還原說明，見 [安裝文件](https://3m-ui.top/guide/install)。

---

## 從原始碼建置

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

需要本機已安裝相容的 **Mihomo** 二進位，並在設定中指向其路徑。

---

## 設定要點

預設資料與設定大致位於：

| 路徑 | 用途 |
|------|------|
| `/etc/3m-ui/config.yaml` | 面板設定 |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | 節點自簽憑證（**請與資料庫一併備份**） |
| `/var/lib/3m-ui/mihomo/` | Mihomo 工作目錄與 `config.yaml` |

JWT / 憑據金鑰請使用獨立隨機值（≥ 32 位元組），不要使用文件範例。

自訂面板連接埠：`3m-ui config port <port>` 或編輯設定後重啟服務。
新增節點預設隨機填寫 10000～60000 的監聽埠（輕量抽樣並避開已載入節點埠；支援倒序範圍），可手動修改或點擊「隨機埠」。

---

## 架構示意

```text
  Browser (Ant Design + Lucide)
        │  REST + JWT
        ▼
  Gin API  ── Telegram Bot
        │
   Service layer (users / listeners / sub / cluster / …)
        │
   SQLite · certstore · visual config
        │
        ▼
     Mihomo (-t 校验 → 应用 / 回滚)
```

---

## 備份建議

至少備份：

1. `3m-ui.db`
2. `listener-certs/`
3. 面板 `config.yaml` 與 Mihomo 設定目錄

僅更新二進位時，憑證會盡量從磁碟 hydrate；若目錄遺失會重新自簽，用戶端需重新拉取訂閱。

---

## 致謝

特別感謝以下開源專案與社群：

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心代理引擎及 Listener 設定模型
- [acme.sh](https://github.com/acmesh-official/acme.sh) — SSH 選單的憑證申請/部署/續期
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 設定範例與協定參考
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — 面板互動與維運思路參考
- [Gin](https://github.com/gin-gonic/gin) — 後端 HTTP 框架
- [GORM](https://github.com/go-gorm/gorm) — 資料庫 ORM
- [React](https://github.com/facebook/react) — 前端基礎
- [Ant Design](https://github.com/ant-design/ant-design) — UI 元件
- [Lucide](https://github.com/lucide-icons/lucide) — 圖示（`lucide-react`，經 `frontend/src/icons.tsx` 封裝）
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo 分流設定與精簡規則思路參考
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo 規則與策略組方案參考
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — 規則集與 mihomo 分流範本參考
- [Zustand](https://github.com/pmndrs/zustand) — 前端狀態管理
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go、Node.js 以及整個開源社群

貢獻者：[Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## 授權條款

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

本專案自 commit `226c460` 之後採用 **GNU General Public License v3.0**。此前的提交與已發布版本仍受 **Eclipse Public License 2.0（EPL-2.0）** 約束，該授權不可撤回。

[GNU General Public License v3.0](../../LICENSE)

第三方元件見 [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)。

---

---

## 多語言版本

完整翻譯（18 種語言）：見頂部語言連結，或打開主 [README](../../README.md)。
