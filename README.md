# 3M-UI

> Mihomo Web Management Console

> 📖 **文档已迁移至 [3m-ui-docs](https://github.com/kazeyukiro/3m-ui-docs) 仓库（基于 Docusaurus 构建）。** 本仓库的 `docs/` 目录已删除，GitHub Wiki 已清空。

---

# 中文

## 简介

3M-UI 是一个轻量、现代的 Mihomo Web 管理面板，面向自托管 VPS / Linux 服务器。

它统一管理 Mihomo Listener、代理用户、订阅、流量、Telegram Bot，以及配置生成、校验、应用和失败回滚。

## 主要功能

- Mihomo 启动、停止、重启和状态查看
- Listener 创建、编辑、校验、删除
- Schema → Validation → Compiler 配置生成链路
- VLESS、VMess、Trojan、Shadowsocks
- Hysteria2、TUIC、ShadowQuic
- AnyTLS、Snell、Mieru、Sudoku、TrustTunnel
- VLESS Reality / XHTTP 等配置能力
- 用户 UUID / Password 凭据管理
- 用户流量限制、到期时间和启停控制
- Listener 订阅 Token 与客户端 URI
- Telegram Bot 通知和管理
- JWT 认证、AES-GCM 凭据加密、CORS 控制
- 配置应用失败自动回滚
- SQLite + GORM 数据持久化

## 前端

项目仅维护一个官方前端：**React + Ant Design**，位于 `frontend/`。

发布产物为 **纯 Go 静态二进制**（`CGO_ENABLED=0` + modernc SQLite），可在 glibc / musl（Alpine）及新旧发行版上直接运行，无需系统 `libsqlite3`。

支持架构：`amd64`（x86_64）、`arm64`、`armv7`、`armv6`、`386`、`riscv64`、`loong64`、`ppc64le`、`s390x`。

## 架构

```text
React Web UI
          │ REST / HTTP
          ▼
Gin API + JWT Middleware
          │
          ▼
Unified Service Layer
 ┌────┼───────────┐
 │    │           │
Auth Users   Listener/Protocol
 │    │           │
Subscription Telegram Config
 │    │           │
 └────┴──────┬────┘
              │
        SQLite + GORM
              │
              ▼
           Mihomo
```

Web UI、Telegram 和 API 客户端统一经过 Service Layer；高权限操作不会绕过 Mihomo 生命周期控制。

## 配置生命周期

```text
编辑 → 预览 → 校验 → 备份 → 应用
                              │
                       启动 / 重启 Mihomo
                              │
                         健康检查
                         ┌────┴────┐
                       成功        失败
                        │            │
                       保留         回滚
```

## 快速开始

### 环境要求

- Linux VPS / Linux Server
- Go 1.25+
- Node.js 20+
- 兼容的 Mihomo 二进制文件
- SQLite

### 构建后端

```bash
cd backend
go mod tidy
go build -o ../3m-ui ./cmd/server
```

启动：

```bash
./3m-ui --config /etc/3m-ui/config.yaml
```

### 构建默认 Ant Design 前端

```bash
cd frontend
npm install
npm run build
```

### 安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

生产环境执行前请先审查脚本内容。

## 配置示例

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

**不要使用示例 Secret。** JWT Secret 和 Credential Key 都应使用独立、随机、至少 32 字节的值。

## Telegram

1. 使用 BotFather 创建 Bot。
2. 将 Bot 加入目标聊天。
3. 获取 Chat ID。
4. 在 3M-UI 设置中配置 Bot Token 和允许的 Chat ID。
5. 使用连接测试确认配置。
6. 发送 `/start` 打开菜单。

管理员操作会在 Callback / 向导阶段再次检查权限。

## 订阅

订阅 Token 是 Bearer 凭据，应像密码一样保护。

典型地址：

```text
/api/v1/client/sub/<token>
```

Token 泄露后应立即禁用或删除。

## 安全建议

- 生产环境使用 HTTPS。
- JWT Secret 与 Credential Key 分开保存。
- `cors_origins` 只允许可信来源。
- 不要泄露 Bot Token、JWT Secret、Credential Key 或订阅 URL。
- 保护 Mihomo 配置和二进制文件。
- 尽可能将管理面板放在防火墙或私有网络之后。

## 开发与测试

后端测试：

```bash
cd backend
go test ./...
```

静态检查：

```bash
gofmt -w path/to/file.go
cd backend
go vet ./...
```

前端构建：

```bash
cd frontend
npm run build
```

## 致谢

特别感谢以下开源项目和社区：

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心代理引擎及 Listener 配置模型。
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Mihomo Listener 配置示例及协议参考；本项目的配置兼容性审查参考了其中的示例。
- [Gin](https://github.com/gin-gonic/gin) — 后端 HTTP 框架。
- [GORM](https://github.com/go-gorm/gorm) — 数据库 ORM。
- [React](https://github.com/facebook/react) — 前端基础框架。
- [Ant Design](https://github.com/ant-design/ant-design) — UI 组件系统。
- [Zustand](https://github.com/pmndrs/zustand) — 前端状态管理。
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT 实现。
- Go、Node.js 以及整个开源社区。

## 许可证

Eclipse Public License 2.0 (EPL-2.0)，详见 [`LICENSE`](LICENSE)。

---

# English

## Introduction

3M-UI is a lightweight, modern web management console for Mihomo, designed for self-hosted VPS and Linux servers.

It provides one interface for managing Mihomo listeners, proxy users, subscriptions, traffic, Telegram administration, and configuration generation, validation, activation and rollback.

## Features

- Mihomo start, stop, restart and status management
- Listener creation, editing, validation and deletion
- Schema → Validation → Compiler configuration pipeline
- VLESS, VMess, Trojan and Shadowsocks
- Hysteria2, TUIC and ShadowQuic
- AnyTLS, Snell, Mieru, Sudoku and TrustTunnel
- VLESS Reality / XHTTP configuration support
- UUID / password credential management
- User traffic limits, expiration and enable/disable lifecycle
- Listener-bound subscription tokens and client URI generation
- Telegram Bot notifications and administration
- JWT authentication, AES-GCM credential encryption and CORS controls
- Automatic configuration rollback after failed activation
- SQLite + GORM persistence

## Frontend

The project maintains one official frontend: **React + Ant Design**, located in `frontend/`.

Release artifacts are **pure-Go static binaries** (`CGO_ENABLED=0` + modernc SQLite). They run on glibc and musl (Alpine) without a system `libsqlite3`.

Architectures: `amd64` (x86_64), `arm64`, `armv7`, `armv6`, `386`, `riscv64`, `loong64`, `ppc64le`, `s390x`.

## Architecture

```text
React Web UI
          │ REST / HTTP
          ▼
Gin API + JWT Middleware
          │
          ▼
Unified Service Layer
 ┌────┼───────────┐
 │    │           │
Auth Users   Listener/Protocol
 │    │           │
Subscription Telegram Config
 │    │           │
 └────┴──────┬────┘
              │
        SQLite + GORM
              │
              ▼
           Mihomo
```

Web UI, Telegram and API clients use the same service layer. Privileged operations do not bypass Mihomo lifecycle controls.

## Configuration lifecycle

```text
Edit → Preview → Validate → Backup → Apply
                                      │
                              Start / Restart Mihomo
                                      │
                                Health check
                                 ┌────┴────┐
                               Success   Failure
                                  │         │
                                 Keep    Rollback
```

## Quick start

### Requirements

- Linux VPS / Linux server
- Go 1.25+
- Node.js 20+
- A compatible Mihomo binary
- SQLite

### Build backend

```bash
cd backend
go mod tidy
go build -o ../3m-ui ./cmd/server
```

Start:

```bash
./3m-ui --config /etc/3m-ui/config.yaml
```

### Build the default Ant Design frontend

```bash
cd frontend
npm install
npm run build
```

### Installer

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

Review the script before running it in production.

## Configuration example

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

**Do not use the example secrets.** JWT and credential-encryption keys should be separate, random values of at least 32 bytes.

## Telegram

1. Create a Bot with BotFather.
2. Add it to the target chat.
3. Obtain the Chat ID.
4. Configure the Bot Token and allowed Chat ID in 3M-UI.
5. Run the connection test.
6. Send `/start` to open the menu.

Privileged actions perform permission checks again during callbacks and wizard flows.

## Subscriptions

Subscription tokens are bearer credentials and should be protected like passwords.

Typical endpoint:

```text
/api/v1/client/sub/<token>
```

Revoke or disable the token immediately if it is exposed.

## Security recommendations

- Use HTTPS in production.
- Keep JWT and credential-encryption keys separate.
- Restrict `cors_origins` to trusted origins.
- Never expose Bot Tokens, JWT secrets, credential keys or subscription URLs.
- Protect the Mihomo configuration and binary from untrusted local users.
- Put the management panel behind a firewall or private network when possible.

## Development and testing

Backend tests:

```bash
cd backend
go test ./...
```

Static checks:

```bash
gofmt -w path/to/file.go
cd backend
go vet ./...
```

Frontend build:

```bash
cd frontend
npm run build
```

## Acknowledgements

Special thanks to the following open-source projects and communities:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core proxy engine and Listener configuration model.
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Mihomo Listener configuration examples and protocol references used during the configuration compatibility review.
- [Gin](https://github.com/gin-gonic/gin) — backend HTTP framework.
- [GORM](https://github.com/go-gorm/gorm) — database ORM.
- [React](https://github.com/facebook/react) — frontend foundation.
- [Ant Design](https://github.com/ant-design/ant-design) — UI component system.
- [Zustand](https://github.com/pmndrs/zustand) — frontend state management.
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT implementation.
- The Go, Node.js and wider open-source communities.

## License

Eclipse Public License 2.0 (EPL-2.0). See [`LICENSE`](LICENSE).
