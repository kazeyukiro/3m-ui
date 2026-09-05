# 3m-ui

**Mihomo 服务端 Web 管理面板**

轻量、自托管，用于在 Linux 上管理 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener、用户、订阅与运行状态。

[![Release](https://img.shields.io/github/v/release/kazeyukiro/3m-ui?include_prereleases)](https://github.com/kazeyukiro/3m-ui/releases)
[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](./backend/go.mod)

> 文档站点（若已发布）：[3m-ui-docs](https://github.com/kazeyukiro/3m-ui-docs) · Wiki：项目 Issues / Discussions

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
| **安全** | JWT、首登改密、凭据加密、CORS；安装后请立即修改默认管理员密码 |

前端：**React + Ant Design**（`frontend/`），构建后嵌入单一 Go 二进制。

发布产物：**纯静态链接**（`CGO_ENABLED=0` + modernc SQLite），无需系统 `libsqlite3`，兼容 glibc / musl。

架构：`linux/amd64` · `arm64` · `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`

---

## 快速安装

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

生产环境请先审查脚本。非交互安装可设置 `THREE_M_UI_NONINTERACTIVE=1`（结果写入 `/etc/3m-ui/install-result.env`）。

安装完成后：

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
默认密码   admin   ← 首次登录必须修改
管理命令   3m-ui
```

更新：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/update.sh | bash
# 或: 3m-ui update
```

卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/uninstall.sh | bash
```

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

## 许可证

[Eclipse Public License 2.0](./LICENSE)

第三方组件见 [THIRD-PARTY-NOTICES.md](./THIRD-PARTY-NOTICES.md)。

---

## English

**3m-ui** is a lightweight, self-hosted web panel for managing a [Mihomo](https://github.com/MetaCubeX/mihomo) server: listeners, users, subscriptions, traffic, Telegram alerts, multi-node registry, and safe config apply/rollback.

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

Default panel: `http://SERVER_IP:8080/` — user `admin` / password `admin` (**change on first login**).

### Build

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Static Linux binaries are published on the [Releases](https://github.com/kazeyukiro/3m-ui/releases) page for multiple architectures.

### License

Eclipse Public License 2.0 — see [LICENSE](./LICENSE).
