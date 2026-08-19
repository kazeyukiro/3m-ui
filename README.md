# 3M-UI

> Mihomo Web Management Console

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

## 多套前端

3M-UI 的后端、API 和配置模型与 UI 解耦。除了默认的 Ant Design 版，还提供独立的 Material UI、Mantine 和 shadcn-style 前端版本。

| 版本 | 技术栈 | GitHub Actions |
|---|---|---|
| Ant Design | React + Ant Design | `release-ant.yml` |
| Material | React + MUI | `release-mui.yml` |
| Mantine | React + Mantine | `release-mantine.yml` |
| shadcn-style | React + Tailwind CSS | `release-shadcn.yml` |

每个版本独立打包 Linux `amd64`、`arm64` 和 `armv7`。选择的前端只在 CI 构建目录中嵌入 Go 二进制，不会把生成产物自动提交回 `main`。

### 发布（推送标签）

推送任意 `v*` 标签会触发统一 **Release** 工作流，一次构建全部 UI 版本，并挂到**同一个** GitHub Release 下：

| 产物后缀 | 前端 |
|---|---|
| `*-antd` | Ant Design（主版） |
| `*-mui` | Material UI |
| `*-mantine` | Mantine |
| `*-shadcn` | shadcn |

每个版本均提供 `linux-amd64` / `arm64` / `armv7`。

```bash
git tag v0.1.0
git push origin v0.1.0
# → 一个 Release，内含 12 个 tar.gz（4 前端 × 3 架构）
```

也可在 Actions 中手动运行 **Release**。单前端重建仍可用 `Release · Ant Design` 等独立 workflow（仅 `workflow_dispatch`）。

详细说明见 [`frontend-variants/README.md`](frontend-variants/README.md)。

## 架构

```text
React Web UI / UI Editions
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

### 构建其他前端

```bash
cd frontend-variants/mui
npm install && npm run build

cd ../mantine
npm install && npm run build

cd ../shadcn
npm install && npm run build
```

每个版本都共享 `frontend-variants/shared/api.ts` 的认证、Listener、用户、Cluster、配置和 Mihomo API 契约。

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

每个替代前端都有独立的 Release Action，不再由一个 Workflow 混合打包所有 UI。

## 致谢

特别感谢以下开源项目和社区：

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心代理引擎及 Listener 配置模型。
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Mihomo Listener 配置示例及协议参考；本项目的配置兼容性审查参考了其中的示例。
- [Gin](https://github.com/gin-gonic/gin) — 后端 HTTP 框架。
- [GORM](https://github.com/go-gorm/gorm) — 数据库 ORM。
- [React](https://github.com/facebook/react) — 前端基础框架。
- [Ant Design](https://github.com/ant-design/ant-design) — UI 组件系统。
- [MUI](https://github.com/mui/material-ui) — Material UI 组件系统。
- [Mantine](https://github.com/mantinedev/mantine) — React UI 组件库。
- [Tailwind CSS](https://github.com/tailwindlabs/tailwindcss) — shadcn-style 前端的样式基础。
- [Zustand](https://github.com/pmndrs/zustand) — 前端状态管理。
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT 实现。
- Go、Node.js 以及整个开源社区。

## 许可证

Apache License 2.0，详见 [`LICENSE`](LICENSE)。

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

## Multiple Frontend Editions

The 3M-UI backend, API contract and configuration model are decoupled from the UI. In addition to the default Ant Design edition, the repository provides independent Material UI, Mantine and shadcn-style editions.

| Edition | Stack | GitHub Actions |
|---|---|---|
| Ant Design | React + Ant Design | `release-ant.yml` |
| Material | React + MUI | `release-mui.yml` |
| Mantine | React + Mantine | `release-mantine.yml` |
| shadcn-style | React + Tailwind CSS | `release-shadcn.yml` |

Each edition produces separate Linux `amd64`, `arm64` and `armv7` packages. The selected frontend is embedded only in the CI workspace before the Go binary is built; generated assets are not automatically committed back to `main`.

### Release (push tags)

Pushing any `v*` tag runs the unified **Release** workflow: all UI editions are built and attached to **one** GitHub Release:

| Artifact suffix | Frontend |
|---|---|
| `*-antd` | Ant Design (primary) |
| `*-mui` | Material UI |
| `*-mantine` | Mantine |
| `*-shadcn` | shadcn |

Each edition ships `linux-amd64` / `arm64` / `armv7`.

```bash
git tag v0.1.0
git push origin v0.1.0
# → one Release with 12 tar.gz files (4 frontends × 3 archs)
```

You can also run **Release** manually in Actions. Single-edition rebuilds remain available via the `Release · …` workflows (`workflow_dispatch` only).

See [`frontend-variants/README.md`](frontend-variants/README.md) for details.

## Architecture

```text
React Web UI / UI Editions
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

### Build alternative editions

```bash
cd frontend-variants/mui
npm install && npm run build

cd ../mantine
npm install && npm run build

cd ../shadcn
npm install && npm run build
```

All editions share `frontend-variants/shared/api.ts` for authentication, listeners, users, cluster, configuration and Mihomo process APIs.

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

Each alternative frontend has its own release Action; releases are no longer mixed into one UI-agnostic packaging workflow.

## Acknowledgements

Special thanks to the following open-source projects and communities:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core proxy engine and Listener configuration model.
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Mihomo Listener configuration examples and protocol references used during the configuration compatibility review.
- [Gin](https://github.com/gin-gonic/gin) — backend HTTP framework.
- [GORM](https://github.com/go-gorm/gorm) — database ORM.
- [React](https://github.com/facebook/react) — frontend foundation.
- [Ant Design](https://github.com/ant-design/ant-design) — UI component system.
- [MUI](https://github.com/mui/material-ui) — Material UI component system.
- [Mantine](https://github.com/mantinedev/mantine) — React UI component library.
- [Tailwind CSS](https://github.com/tailwindlabs/tailwindcss) — styling foundation for the shadcn-style edition.
- [Zustand](https://github.com/pmndrs/zustand) — frontend state management.
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT implementation.
- The Go, Node.js and wider open-source communities.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
