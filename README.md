# 3m-ui

<p align="center">
  <img src="frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

**Mihomo 服务端 Web 管理面板**


> **稳定版优先：** 默认安装和更新只使用正式 Release。预发布需要主动选择。完整安装包及 Docker 镜像包含固定版本的 Mihomo；详见 [安装、升级与恢复](docs/installation.md)。

轻量、自托管，用于在 Linux 上管理 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener、用户、订阅与运行状态。

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)
<a href="https://linux.do" alt="LINUX DO">
      <img src="https://img.shields.io/badge/LINUX-DO-FFB003.svg?logo=data:image/svg%2bxml;base64,DQo8c3ZnIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgd2lkdGg9IjEwMCIgaGVpZ2h0PSIxMDAiPjxwYXRoIGQ9Ik00Ni44Mi0uMDU1aDYuMjVxMjMuOTY5IDIuMDYyIDM4IDIxLjQyNmM1LjI1OCA3LjY3NiA4LjIxNSAxNi4xNTYgOC44NzUgMjUuNDV2Ni4yNXEtMi4wNjQgMjMuOTY4LTIxLjQzIDM4LTExLjUxMiA3Ljg4NS0yNS40NDUgOC44NzRoLTYuMjVxLTIzLjk3LTIuMDY0LTM4LjAwNC0yMS40M1EuOTcxIDY3LjA1Ni0uMDU0IDUzLjE4di02LjQ3M0MxLjM2MiAzMC43ODEgOC41MDMgMTguMTQ4IDIxLjM3IDguODE3IDI5LjA0NyAzLjU2MiAzNy41MjcuNjA0IDQ2LjgyMS0uMDU2IiBzdHlsZT0ic3Ryb2tlOm5vbmU7ZmlsbC1ydWxlOmV2ZW5vZGQ7ZmlsbDojZWNlY2VjO2ZpbGwtb3BhY2l0eToxIi8+PHBhdGggZD0iTTQ3LjI2NiAyLjk1N3EyMi41My0uNjUgMzcuNzc3IDE1LjczOGE0OS43IDQ5LjcgMCAwIDEgNi44NjcgMTAuMTU3cS00MS45NjQuMjIyLTgzLjkzIDAgOS43NS0xOC42MTYgMzAuMDI0LTI0LjM4N2E2MSA2MSAwIDAgMSA5LjI2Mi0xLjUwOCIgc3R5bGU9InN0cm9rZTpub25lO2ZpbGwtcnVsZTpldmVub2RkO2ZpbGw6IzE5MTkxOTtmaWxsLW9wYWNpdHk6MSIvPjxwYXRoIGQ9Ik03Ljk4IDcwLjkyNmMyNy45NzctLjAzNSA1NS45NTQgMCA4My45My4xMTNRODMuNDI2IDg3LjQ3MyA2Ni4xMyA5NC4wODZxLTE4LjgxIDYuNTQ0LTM2LjgzMi0xLjg5OC0xNC4yMDMtNy4wOS0yMS4zMTctMjEuMjYyIiBzdHlsZT0ic3Ryb2tlOm5vbmU7ZmlsbC1ydWxlOmV2ZW5vZGQ7ZmlsbDojZjlhZjAwO2ZpbGwtb3BhY2l0eToxIi8+PC9zdmc+" />
    </a>
> 文档站：[3m-ui-docs](https://github.com/kazeyukiro/3m-ui-docs) 

---

## 特性

| 类别 | 能力 |
|------|------|
| **节点** | 协议注册表驱动的 Listener：VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC 等；Reality、TLS 自签（落盘可恢复）、传输层字段互斥校验 |
| **用户** | 绑定节点、流量限额、到期、IP 限制、批量启停 / 延期 / 加量、订阅 Token |
| **订阅** | UA 自动识别 Clash/Mihomo YAML、v2ray Base64、sing-box JSON；可选 `?target=`；HTML 订阅页 |
| **配置** | 生成 → 校验 → 应用 分离；失败回滚上一份 `config.yaml` |
| **运维** | 核心启停/更新、日志、仪表盘资源占用、Geo 文件、面板 SSL/ACME、备份恢复 |
| **Telegram** | 告警与管理命令（需 Token + Chat ID） |
| **多机** | 登记远程面板、健康检查、同步节点镜像、合并订阅、可选推送节点（默认远程禁用） |
| **安全** | JWT、首登改密、凭据加密、CORS；随机初始密码、首登改密 |

前端：**React + Ant Design**（`frontend/`），构建后嵌入单一 Go 二进制。

发布产物：**纯静态链接**（`CGO_ENABLED=0` + modernc SQLite），无需系统 `libsqlite3`，兼容 glibc / musl。

完整安装包 / Docker：`linux/amd64` · `arm64`。高级独立面板二进制另外支持 `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`，需要自行准备兼容内核。

---

## 快速安装

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

完整安装支持 Linux amd64 / arm64，自动安装面板、配套 Mihomo 和系统服务。首次配置和管理员密码由程序统一生成；配置已存在时保持原值。其他架构可使用独立二进制和自行准备的内核，见 [详细安装说明](docs/installation.md)。

安装完成后：

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
# 指定版本: sudo 3m-ui update v1.0.0
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

卸载：

```bash
sudo 3m-ui uninstall
```

---

## Docker 安装

Linux VPS 下载发布版本中的 `docker-compose.yml`，在同一目录执行：

```bash
docker compose up -d
docker compose logs 3m-ui
```

镜像为 `ghcr.io/kazeyukiro/3m-ui:latest`，包含面板及固定版本的 Mihomo；`latest` 只跟随稳定版。配置、随机密钥和管理员会自动初始化，首次密码只在首次启动输出一次。默认 host 网络，新增节点后放行实际使用的 TCP/UDP 端口。另提供 `docker-compose.bridge.yml`，以及版本固定、证书配置和持久化恢复说明，见 [安装文档](docs/installation.md)。

---

## 从源码构建

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

需要本机已安装兼容的 **Mihomo** 二进制，并在配置中指向其路径。

---

## 配置要点

默认数据与配置大致位于：

| 路径 | 用途 |
|------|------|
| `/etc/3m-ui/config.yaml` | 面板配置 |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | 节点自签证书（**请与数据库一并备份**） |
| `/var/lib/3m-ui/mihomo/` | Mihomo 工作目录与 `config.yaml` |

JWT / 凭据密钥请使用独立随机值（≥ 32 字节），不要使用文档示例。

自定义面板端口：`3m-ui config port <port>` 或编辑配置后重启服务。
新建节点默认随机填写 10000～60000 的监听端口（轻量抽样并避开已加载节点端口；支持倒序范围），可手动修改或点击「随机端口」。


---

## 架构示意

```text
  Browser (Ant Design)
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

## 备份建议

至少备份：

1. `3m-ui.db`
2. `listener-certs/`
3. 面板 `config.yaml` 与 Mihomo 配置目录

仅更新二进制时，证书会尽量从磁盘 hydrate；若目录丢失会重新自签，客户端需重新拉取订阅。

---


## 致谢

特别感谢以下开源项目和社区：

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心代理引擎及 Listener 配置模型
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 配置示例与协议参考
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — 面板交互与运维思路参考
- [Gin](https://github.com/gin-gonic/gin) — 后端 HTTP 框架
- [GORM](https://github.com/go-gorm/gorm) — 数据库 ORM
- [React](https://github.com/facebook/react) — 前端基础
- [Ant Design](https://github.com/ant-design/ant-design) — UI 组件
- [Zustand](https://github.com/pmndrs/zustand) — 前端状态管理
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go、Node.js 以及整个开源社区

贡献者：[kazeyukiro](https://github.com/kazeyukiro)、[freephilx](https://github.com/freephilx)

---

## 许可证

[Eclipse Public License 2.0](./LICENSE)

第三方组件见 [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md)。

---

## English

**3m-ui** is a lightweight, self-hosted web panel for managing a [Mihomo](https://github.com/MetaCubeX/mihomo) server: listeners, users, subscriptions, traffic, Telegram alerts, multi-node registry, and safe config apply/rollback.

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Update after install: `sudo 3m-ui update` (optional tag: `sudo 3m-ui update v1.0.0`).

Default panel: `http://SERVER_IP:8080/` — user `admin` with a randomly generated initial password, printed once (**change on first login**). Native bundles and Docker images include the pinned Mihomo core. See [installation, upgrade and recovery](docs/installation.md).

### Build

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Static Linux binaries are published on the [Releases](https://github.com/kazeyukiro/3m-ui/releases) page for multiple architectures.


### Acknowledgements

Special thanks to:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — core engine and listener model
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — listener examples and protocol references
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel UX and ops inspiration
- [Gin](https://github.com/gin-gonic/gin), [GORM](https://github.com/go-gorm/gorm), [React](https://github.com/facebook/react), [Ant Design](https://github.com/ant-design/ant-design), [Zustand](https://github.com/pmndrs/zustand), [golang-jwt/jwt](https://github.com/golang-jwt/jwt)
- The Go, Node.js, and wider open-source communities

Contributors: [kazeyukiro](https://github.com/kazeyukiro), [freephilx](https://github.com/freephilx).

### License

Eclipse Public License 2.0 — see [LICENSE](./LICENSE).
