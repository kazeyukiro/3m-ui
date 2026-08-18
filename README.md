# 3M-UI

**Mihomo Management Console · Mihomo 管理面板**

3M-UI is a lightweight web management console for [Mihomo](https://github.com/MetaCubeX/mihomo). It provides a unified interface for managing listeners, proxy users, subscriptions, traffic, configuration lifecycle, and Telegram administration.

3M-UI 是一个轻量、现代的 Mihomo Web 管理面板，用于统一管理 Listener 入站、代理用户、订阅、流量统计、配置生命周期以及 Telegram 管理。

> **Status / 状态:** Active development / 持续开发中
>
> This project is intended for self-hosted deployments. Review the configuration and security settings before exposing the panel to the Internet.
>
> 本项目面向自托管部署。将面板暴露到公网前，请务必检查认证、HTTPS、CORS 和订阅 Token 等安全配置。

---

## ✨ What it does / 功能

### Mihomo management / Mihomo 管理

- Start, stop, restart and inspect Mihomo
- Create, edit, validate and delete Listener configurations
- Generate Mihomo configuration from structured Listener data
- Preview and validate configuration before activation
- Automatic backup and rollback when configuration activation fails
- View generated configuration and Mihomo logs
- Listener/user relationships are persisted in SQLite

### Protocol & subscription management / 协议与订阅

- VLESS, VMess, Trojan, Shadowsocks
- Hysteria2, TUIC, ShadowQuic
- AnyTLS, Snell, Mieru, Sudoku, TrustTunnel and related Mihomo Listener types
- VLESS Reality and XHTTP configuration fields
- Credential-aware user generation
- Client URI generation
- Listener-bound subscription tokens
- Subscription expiry and enable/disable lifecycle

The project keeps protocol configuration in a schema/validation/compiler pipeline so that the Web UI does not have to construct raw Mihomo YAML by hand.

项目采用 **Schema → Validation → Compiler → Mihomo YAML** 的配置链路，避免前端直接拼接原始 Mihomo YAML。

### Client management / 客户端管理

- Proxy user credentials
- UUID/password based credentials depending on protocol
- Traffic limits and expiry
- Enable/disable lifecycle
- Online status and traffic information
- Telegram account binding

### Telegram Bot / Telegram 机器人

The built-in Telegram Bot supports both notifications and interactive administration.

内置 Telegram Bot 同时支持通知和交互式管理。

- Bilingual notifications / 中英双语通知
- Expiry, traffic-limit, blocked and restored notifications
- Optional daily usage summary
- Bot token validation
- Inline keyboard administration
- User ↔ Telegram ID binding
- User self-service usage and node information
- Administrator views for users, online users, listeners and traffic
- Mihomo restart through the central service layer
- Client creation wizard
- Permission checks are repeated for privileged callback actions
- Telegram update offset persistence across restarts

### Security / 安全

- JWT authentication
- JWT algorithm validation
- Database-backed user/session checks
- AES-GCM encryption for recoverable credentials
- Configurable CORS origins
- Login failure throttling
- Subscription token lifecycle controls
- Configuration validation before activation
- Automatic configuration rollback on activation failure

---

## 🏗️ Architecture / 架构

```text
                    ┌──────────────────────┐
                    │   React Web UI       │
                    │   Zustand + AntD     │
                    └──────────┬───────────┘
                               │ REST / HTTP
                    ┌──────────▼───────────┐
                    │      Gin API         │
                    │   JWT + Middleware   │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   Service Layer      │
                    ├──────────────────────┤
                    │ Auth / Users         │
                    │ Listeners / Protocol │
                    │ Subscription         │
                    │ Telegram             │
                    │ Config / Mihomo      │
                    └───────┬───────┬──────┘
                            │       │
                     ┌──────▼───┐  │
                     │ SQLite   │  │
                     │ + GORM  │  │
                     └──────────┘  │
                                   ▼
                            ┌────────────┐
                            │   Mihomo   │
                            │    Core    │
                            └────────────┘
```

The important design rule is that Web UI, Telegram and API clients use the same backend service layer. Privileged operations should not bypass Mihomo lifecycle and configuration controls.

核心设计原则是：Web UI、Telegram 和 API 客户端统一经过后端 Service Layer；高权限操作不能绕过 Mihomo 生命周期和配置控制。

---

## 🚀 Quick start / 快速开始

### Requirements / 环境要求

- Linux VPS or Linux server / Linux VPS 或 Linux 服务器
- Go 1.25+
- Node.js 20+
- A compatible Mihomo binary
- SQLite

### Build backend / 构建后端

```bash
cd backend
go mod tidy
go build -o ../3m-ui ./cmd/server
```

Start the server with your configuration file:

```bash
./3m-ui --config /etc/3m-ui/config.yaml
```

### Build frontend / 构建前端

```bash
cd frontend
npm install
npm run build
```

For local development:

```bash
npm run dev
```

### Installer / 安装脚本

The repository also provides an installation script:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

Review the script before piping it to a shell in production environments.

生产环境执行安装脚本前，建议先阅读并审查脚本内容。

---

## ⚙️ Configuration / 配置

The application configuration is loaded from YAML. At minimum, provide a database path, a unique JWT secret and a unique credential encryption key.

应用配置使用 YAML。至少需要配置数据库路径、唯一的 JWT Secret，以及唯一的凭据加密密钥。

Example:

```yaml
server:
  port: 8080
  public_url: "https://panel.example.com"

database:
  path: "/etc/3m-ui/3m-ui.db"

jwt:
  secret: "REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_BYTES"

security:
  credential_key: "REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_BYTES"
  cors_origins:
    - "https://panel.example.com"

mihomo:
  binary: "/usr/local/bin/mihomo"
  config: "/etc/mihomo/config.yaml"
```

**Do not use the example secrets.** The application rejects the built-in placeholder secrets and requires secrets of at least 32 bytes.

**不要使用示例中的 Secret。** 应用会拒绝内置占位 Secret，并要求 Secret 至少 32 字节。

Generate strong secrets with a password manager or a cryptographically secure random generator.

---

## 🔗 Subscriptions / 订阅

Subscriptions use bearer tokens. A token is associated with the listener/user data exposed by the subscription and can be independently disabled or expired.

订阅使用 Bearer Token。Token 与订阅所暴露的 Listener/用户数据关联，并可以独立禁用或设置到期时间。

Typical endpoint:

```text
/api/v1/client/sub/<token>
```

Supported output targets depend on the current subscription implementation. Do not assume that every client format is enabled in every deployment.

订阅支持的输出格式以当前版本实现为准，不要假设所有客户端格式在每个部署中都默认启用。

Treat subscription URLs as passwords. If a URL is exposed, revoke or disable the corresponding token.

请把订阅 URL 当作密码处理；一旦泄露，应立即禁用或删除对应 Token。

---

## 🤖 Telegram setup / Telegram 配置

1. Create a bot with Telegram's **BotFather** and copy the Bot Token.
2. Add the bot to the target chat.
3. Obtain the target Chat ID.
4. Open **Settings → Telegram** in 3M-UI.
5. Configure the Bot Token and allowed Chat ID.
6. Run the built-in connection test.
7. Send `/start` to open the bot menu.

1. 使用 Telegram **BotFather** 创建机器人并复制 Bot Token。
2. 将机器人加入目标聊天。
3. 获取目标 Chat ID。
4. 打开 3M-UI 的 **设置 → Telegram**。
5. 配置 Bot Token 和允许的 Chat ID。
6. 使用面板提供的测试功能检查连接。
7. 向机器人发送 `/start` 打开菜单。

### Commands / 命令

| Command | Description | Permission |
|---|---|---|
| `/start` | Open the menu / 打开菜单 | All |
| `/help` | Show help / 查看帮助 | All |
| `/status` | Mihomo status / 核心状态 | All |
| `/id` | Show Telegram ID / 查看 Telegram ID | All |
| `/usage` | Usage information / 用量信息 | All |
| `/inbound <remark>` | Listener information / 入站信息 | Admin |
| `/restart` | Restart Mihomo / 重启核心 | Admin |
| `/bind <username> <telegram_id>` | Bind a client / 绑定客户端 | Admin |
| `/users` | List clients / 客户端列表 | Admin |
| `/online` | Online clients / 在线客户端 | Admin |
| `/listeners` | Listener list / 入站列表 | Admin |
| `/traffic` | Traffic ranking / 流量排行 | Admin |
| `/cancel` | Cancel current wizard / 取消当前向导 | All |

Privileged actions perform permission checks again when handling callbacks and wizard actions.

高权限操作在处理 Callback 和向导动作时会再次进行权限检查。

---

## 🧩 Configuration lifecycle / 配置生命周期

3M-UI treats configuration activation as a controlled operation rather than simply overwriting the live Mihomo YAML.

3M-UI 不会简单覆盖正在使用的 Mihomo YAML，而是把配置应用视为受控生命周期操作。

```text
Edit / 编辑
   │
   ▼
Preview / 预览
   │
   ▼
Validate / 校验
   │
   ▼
Backup / 备份
   │
   ▼
Apply / 应用
   │
   ▼
Start / Restart Mihomo
启动 / 重启 Mihomo
   │
   ▼
Health check / 健康检查
   │
   ├── Success → keep
   │              成功 → 保留
   │
   └── Failure → rollback
                  失败 → 回滚
```

This reduces the chance of leaving a deployment with a broken active configuration after an unsuccessful restart.

这样可以降低 Mihomo 重启失败后留下不可用配置的风险。

---

## 🔐 Security recommendations / 安全建议

- Use HTTPS in production, preferably behind Nginx or Caddy.
- Use a unique JWT secret of at least 32 bytes.
- Use a separate credential encryption key of at least 32 bytes.
- Restrict `cors_origins` to trusted origins.
- Never publish JWT secrets, credential keys, Telegram Bot Tokens or subscription URLs.
- Protect the Mihomo configuration and binary from untrusted local users.
- Treat Telegram Chat IDs and subscription tokens as access-control data.
- Put the panel behind a firewall or private network when possible.
- Review the installation script before executing it on a production host.

- 生产环境使用 HTTPS，建议通过 Nginx 或 Caddy 反向代理。
- JWT Secret 至少 32 字节，并使用唯一随机值。
- Credential Key 至少 32 字节，并与 JWT Secret 分开。
- `cors_origins` 仅允许可信来源。
- 不要泄露 JWT Secret、Credential Key、Telegram Bot Token 或订阅 URL。
- 保护 Mihomo 配置文件和二进制文件，避免被不可信的本地用户修改。
- Telegram Chat ID 和订阅 Token 应按访问控制数据处理。
- 条件允许时，将管理面板放在防火墙或私有网络之后。
- 在生产主机执行安装脚本前先审查脚本内容。

---

## 🧪 Development / 开发

### Backend tests / 后端测试

```bash
cd backend
go test ./...
```

### Static checks / 静态检查

```bash
gofmt -w backend
cd backend
go vet ./...
```

### Frontend / 前端

```bash
cd frontend
npm run build
```

Before submitting a change, run the backend tests and frontend production build. CI also performs repository-level validation.

提交修改前，请至少运行后端测试和前端生产构建；CI 还会执行仓库级别的校验。

---

## 🛠️ Project structure / 项目结构

```text
3m-ui/
├── backend/              # Go backend / Go 后端
│   ├── cmd/               # Application entrypoints / 程序入口
│   └── internal/          # Services, API, protocol and Mihomo logic
├── frontend/             # React frontend / React 前端
├── scripts/              # Installation and maintenance scripts
├── LICENSE
└── README.md
```

---

## 🙏 Acknowledgements / 致谢

3M-UI stands on the work of many open-source projects and communities. Special thanks to:

3M-UI 建立在众多开源项目和社区的基础之上，特别感谢：

- **[Mihomo](https://github.com/MetaCubeX/mihomo)** — the core proxy engine and Listener configuration model that 3M-UI manages.
- **[clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/)** — valuable Mihomo Listener configuration examples and protocol references used during configuration compatibility review.
- **[Gin](https://github.com/gin-gonic/gin)** — HTTP web framework for the backend.
- **[GORM](https://github.com/go-gorm/gorm)** — database ORM layer.
- **[React](https://github.com/facebook/react)** — frontend UI foundation.
- **[Ant Design](https://github.com/ant-design/ant-design)** — UI component system.
- **[Zustand](https://github.com/pmndrs/zustand)** — lightweight frontend state management.
- **[golang-jwt/jwt](https://github.com/golang-jwt/jwt)** — JWT implementation used for panel authentication.
- **The Go, Node.js and open-source communities** — for the tools, libraries and documentation that make this project possible.

If a project or contributor has been omitted, please open an issue or pull request and we will be happy to add a proper acknowledgement.

如果遗漏了项目或贡献者，欢迎提交 Issue 或 Pull Request，我们会补充准确的致谢信息。

---

## 🤝 Contributing / 参与贡献

Bug reports, protocol compatibility fixes, documentation improvements and code contributions are welcome.

欢迎提交 Bug、协议兼容性修复、文档改进以及代码贡献。

When reporting a problem, include the relevant 3M-UI version, Mihomo version, configuration snippet (with secrets removed), logs and reproduction steps whenever possible.

提交问题时，如条件允许，请提供 3M-UI 版本、Mihomo 版本、脱敏后的配置片段、日志以及复现步骤。

---

## 📄 License / 许可证

3M-UI is released under the **MIT License**. See [`LICENSE`](LICENSE) for the full text.

3M-UI 使用 **MIT License**，完整许可文本请参阅 [`LICENSE`](LICENSE)。
