# 3M-UI — Mihomo Management Console

<p align="center">
  <strong>English</strong> · <a href="#中文">中文</a>
</p>

3M-UI is a lightweight web management console for <a href="https://github.com/MetaCubeX/mihomo">Mihomo Core</a>. It focuses on VPS node management, listener configuration, proxy-user credentials, traffic usage, client subscriptions, configuration lifecycle management, and Telegram operations.

3M-UI 是一个轻量、现代的 Mihomo Core Web 管理面板，主要用于 VPS 节点管理、Listener 入站配置、代理用户凭据、流量统计、客户端订阅、配置生命周期管理以及 Telegram 管理与通知。

---

## ✨ Features / 功能

### Core / 核心管理

- Mihomo start / stop / restart / status
- Listener create, update, validation and configuration regeneration
- SQLite + GORM storage
- JWT-based panel authentication
- Client URI and subscription generation
- Credential-aware Mihomo configuration generation
- Configuration preview and validation before activation
- Safe configuration apply with automatic `.bak` backup
- Configuration rollback through the Mihomo service layer
- Subscription token enable/disable, expiry and lifecycle management

### Telegram / Telegram 机器人

Telegram is more than a notification channel: the built-in Bot supports both notifications and interactive administration.

Telegram 不只是通知渠道，内置 Bot 同时支持通知、查询和交互式管理。

- Bilingual notifications: Chinese + English / 中英双语通知
- User blocked, expired, traffic-limit and restored notifications
- Optional daily usage summary / 可选每日流量摘要
- Telegram Bot Token validation with `getMe`
- Correct handling of Telegram HTTP 200 + `ok=false` responses
- Command menu registered automatically at Bot startup
- Inline keyboard menus / Inline 按钮菜单
- Telegram ID → proxy-user binding
- User self-service usage and node links
- Admin views for users, online users, listeners, traffic ranking, blocked users and low-quota users
- Admin `/restart` for Mihomo through the Mihomo service only
- Admin client creation wizard with listener selection, generated password and binding
- Callback permissions are re-checked before privileged actions
- Wizard state prevents duplicate client-creation flows
- Telegram update offset is persisted across bot restarts

### Bot Commands / Bot 命令

| Command | English | 中文 | Permission |
|---|---|---|---|
| `/start` | Open menu | 打开菜单 | All |
| `/help` | Show help | 查看帮助 | All |
| `/status` | Service status | 服务状态 | All |
| `/id` | Telegram ID | 查看 Telegram ID | All |
| `/usage` | Own usage; admins can search users | 查看自己的用量；管理员可查询客户端 | All |
| `/inbound <remark>` | View listener | 查看入站 | Admin |
| `/restart` | Restart Mihomo | 重启 Mihomo | Admin |
| `/bind <username> <telegram_id>` | Bind a client | 绑定客户端 | Admin |
| `/users` | List clients | 客户端列表 | Admin |
| `/online` | Online clients | 在线客户端 | Admin |
| `/listeners` | Listener list | 入站列表 | Admin |
| `/traffic` | Traffic ranking | 流量排行 | Admin |
| `/cancel` | Cancel current wizard | 取消当前向导 | All |

普通用户不能通过 `/usage <other-user>` 查询其他客户端；`/inbound` 和 `/restart` 严格要求管理员权限。

Regular users cannot use `/usage <other-user>` to inspect another client; `/inbound` and `/restart` require administrator privileges.

The Bot also exposes the same major functions through inline keyboards.

Bot 还提供 Inline Keyboard，可直接点击完成主要查询和管理操作。

---

## 🛡️ P0 Reliability / P0 可靠性

The P0 baseline is designed around one rule: **Web UI, Telegram and future API clients must all use the same service layer and never bypass Mihomo lifecycle controls.**

P0 基线遵循一个原则：**Web UI、Telegram 以及未来的 API/CLI 都必须经过统一 Service Layer，不能绕过 Mihomo 生命周期管理。**

- **Subscription / 订阅** — secure listener-bound tokens, expiry checks, Mihomo YAML and optional subconverter formats
- **Client management / 客户端管理** — credentials, traffic limits, expiry, online state and Telegram binding
- **Config preview / 配置预览** — validate a candidate without touching the live configuration
- **Config apply / 配置应用** — validate, back up the current configuration, activate, and restore on failure
- **Config rollback / 配置回滚** — restore the previous `.bak` configuration through `Mihomo Service`
- **Mihomo Service / 核心服务** — start, stop, restart, status, logs and guarded configuration activation
- **Telegram / Telegram** — bilingual notification/admin workflow with permission re-checks and persisted update offset

---

## 🏗️ Architecture / 架构

```text
┌──────────────────────────────┐
│ React 19 + Zustand + AntD 6 │
└──────────────┬───────────────┘
               │ HTTP / REST
┌──────────────▼───────────────┐
│ Gin + GORM + JWT             │
└──────────────┬───────────────┘
               │ Unified Service Layer
       ┌───────┼──────────┬────────────┐
       ▼       ▼          ▼            ▼
   Clients  Telegram  Config Engine  Mihomo Service
       │       │          │            │
       └───────┴──────────┴────────────┘
                        │
                        ▼
                 ┌──────────────┐
                 │ Mihomo Core  │
                 └──────────────┘
```

---

## 🚀 Quick Start / 快速开始

### Requirements / 环境要求

- Go 1.25+
- Node.js 20+
- Linux VPS; Ubuntu/Debian recommended / Linux VPS，推荐 Ubuntu/Debian

### Backend / 后端

```bash
cd backend
go mod tidy
go build -o ../3m-ui ./cmd/server
../3m-ui --config /etc/3m-ui/config.yaml
```

### Frontend / 前端

```bash
cd frontend
npm install
npm run dev
npm run build
```

Production builds are placed under `backend/static/` by the project build process.

生产构建会生成到项目使用的 `backend/static/` 目录。

### One-line install / 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

---

## ⚙️ Configuration / 配置

Example `/etc/3m-ui/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  public_url: "https://your-domain.com"

database:
  path: "/etc/3m-ui/3m-ui.db"

jwt:
  secret: "change-me-to-a-32-char-random-string"

mihomo:
  binary_path: "/usr/local/bin/mihomo"
  config_path: "/etc/mihomo/config.yaml"

security:
  cors_origins: []
```

Telegram settings are configured from the panel. Enable Telegram only after providing a valid Bot Token and at least one Chat ID.

Telegram 设置在面板中完成。启用 Telegram 通知前，需要填写有效的 Bot Token，并至少配置一个 Chat ID。

Available notification switches / 可用通知开关：

- `notify_on_block` — blocked / disabled / traffic-limit notifications / 封禁、禁用、流量耗尽通知
- `notify_on_unblock` — restored notifications / 恢复通知
- `notify_on_expiry` — expiry notifications / 到期通知
- `notify_daily_digest` — daily summary / 每日摘要

---

## 🔗 Subscription / 订阅

Create a listener-bound access token from the panel. The token is the credential for the public subscription endpoint:

```text
/api/v1/client/sub/<token>
```

Optional targets include:

```text
?target=mihomo
?target=clash
?target=sing-box
?target=shadowrocket
```

Subscription tokens can be disabled, expired or deleted without changing the underlying listener configuration.

订阅 Token 可以单独禁用、设置到期时间或删除，不会直接修改底层 Listener 配置。

---

## 🔐 Telegram Setup / Telegram 配置

1. Create a Bot with Telegram's BotFather and copy the Bot Token.
2. Add the Bot to the target chat and obtain the Chat ID.
3. Open **Settings → Telegram** in 3M-UI.
4. Enable Telegram, enter the Token and Chat ID, then save.
5. Use **Test** to verify the Bot connection.
6. Send `/start` to open the Bot menu.

1. 使用 Telegram 的 BotFather 创建机器人并复制 Bot Token。
2. 将机器人加入目标聊天，并获取 Chat ID。
3. 打开 3M-UI 的 **设置 → Telegram**。
4. 开启 Telegram，填写 Token 和 Chat ID 后保存。
5. 点击 **测试** 验证机器人连接。
6. 向机器人发送 `/start` 打开菜单。

For self-service commands, bind a proxy user to the user's Telegram ID. Administrators can use `/bind <username> <telegram_id>`.

如需用户自助查询，需要将代理用户与 Telegram ID 绑定。管理员可以使用 `/bind <username> <telegram_id>` 完成绑定。

---

## 🧩 Configuration Lifecycle / 配置生命周期

Recommended production workflow / 推荐生产流程：

```text
Edit / 编辑
   ↓
Preview / 预览
   ↓
Validate / 校验
   ↓
Apply / 应用
   ↓
Backup current config / 备份旧配置
   ↓
Restart or start Mihomo / 重启或启动核心
   ↓
Health check / 健康检查
   ↓
Success → keep / 成功 → 保留
Failure → rollback / 失败 → 回滚
```

API endpoints / API：

| Method | Endpoint | Description / 说明 |
|---|---|---|
| POST | `/api/v1/config/preview` | Validate candidate without applying / 仅预览校验 |
| POST | `/api/v1/config/validate` | Validate YAML / 校验 YAML |
| POST | `/api/v1/config/apply` | Generate, validate and safely activate / 安全生成、校验并应用 |
| POST | `/api/v1/config/rollback` | Restore previous `.bak` / 恢复上一份配置 |
| GET | `/api/v1/config/download` | Download generated YAML / 下载配置 |

---

## 🛡️ Security / 安全建议

1. The initial administrator credentials remain `admin` / `admin`. Do not change the initialization logic. Change the password after first login if your deployment requires it.
2. Use HTTPS in production, preferably behind Nginx or Caddy.
3. Use a strong JWT secret of at least 32 random characters.
4. Restrict CORS origins to trusted administration domains.
5. Keep the Mihomo binary in an approved system path.
6. Set `public_url` correctly when using a reverse proxy.
7. Never publish your Telegram Bot Token in issues, screenshots or public repositories.
8. Subscription tokens are bearer credentials; treat them like passwords and revoke them when exposed.

1. 初始管理员账号仍为 `admin` / `admin`，不会修改初始化密码逻辑。生产部署建议首次登录后修改密码。
2. 生产环境使用 HTTPS，建议通过 Nginx 或 Caddy 反向代理。
3. JWT Secret 使用至少 32 位随机字符串。
4. CORS 仅允许可信的管理域名。
5. Mihomo 二进制文件放置在受信任的系统路径。
6. 使用反向代理时正确设置 `public_url`。
7. 不要在 Issue、截图或公开仓库中泄露 Telegram Bot Token。
8. 订阅 Token 属于 Bearer 凭据，泄露后应立即禁用或删除。

---

## 📡 API Overview / API 概览

| Method | Endpoint | Auth | Description / 说明 |
|---|---|---|---|
| POST | `/api/v1/auth/login` | No | Login / 登录 |
| POST | `/api/v1/auth/password` | Yes | Change password / 修改密码 |
| GET | `/api/v1/auth/me` | Yes | Current user / 当前用户 |
| GET | `/api/v1/dashboard` | Yes | System and Mihomo status / 系统与核心状态 |
| GET | `/api/v1/nodes` | Yes | List listeners / 入站列表 |
| POST | `/api/v1/nodes` | Yes | Create listener / 创建入站 |
| GET | `/api/v1/nodes/:id/uri` | Yes | Export node URI / 导出节点 URI |
| GET | `/api/v1/client/sub/:token` | Token | Client subscription / 客户端订阅 |
| GET | `/api/v1/telegram/settings` | Yes | Telegram settings / Telegram 设置 |
| POST | `/api/v1/telegram/test` | Yes | Test Telegram / 测试 Telegram |
| POST | `/api/v1/mihomo/start` | Yes | Start Mihomo / 启动核心 |
| POST | `/api/v1/mihomo/stop` | Yes | Stop Mihomo / 停止核心 |
| POST | `/api/v1/mihomo/restart` | Yes | Restart Mihomo / 重启核心 |
| POST | `/api/v1/config/apply` | Yes | Safe config activation / 安全应用配置 |
| POST | `/api/v1/config/rollback` | Yes | Restore previous config / 回滚配置 |

---

## 🧪 Development / 开发

Backend tests:

```bash
cd backend
go test ./...
```

Frontend:

```bash
cd frontend
npm run build
```

Before submitting changes, verify both backend tests and the production frontend build.

提交修改前，请至少验证后端测试和前端生产构建。

---

## 📄 License / 许可证

MIT License. See [`LICENSE`](LICENSE).

MIT License，详见 [`LICENSE`](LICENSE)。
