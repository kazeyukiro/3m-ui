# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · **简体中文** · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo 服务端 Web 管理面板**

> **稳定版优先：** 默认安装和更新只使用正式 Release。预发布需主动选择。完整安装包及 Docker 镜像包含固定版本 Mihomo；详见 [安装文档](https://3m-ui.top/docs/install.html)。

轻量、自托管，用于在 Linux 上管理 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener、用户、订阅与运行状态。

[![License](https://img.shields.io/badge/license-EPL--2.0-blue.svg)](../../LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)

> 文档: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · [Website](https://3m-ui.top/)

---

## 特性

| 类别 | 能力 |
|------|------|
| **节点** | 协议注册表驱动的 Listener：VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC 等；REALITY、TLS 自签、一键批量证书、**流量倍率**、传输层字段校验 |
| **用户** | **一键创建**（随机用户名/密码/UUID，可选绑定全部节点）、绑定节点、流量限额、**按节点分别计量**（计费 = 实际 × 节点倍率）、到期、IP/订阅拉取限制、首次使用起算、周期续期/重置、分组标签、外部订阅合并、批量操作、订阅 Token |
| **订阅** | UA 识别 Clash/Mihomo YAML、v2ray Base64（`?target=v2ray` 始终 Base64）、sing-box JSON；可选 `?target=`；HTML 订阅页；TUIC/HY2 分享链含客户端 TLS 参数 |
| **配置** | 生成 → 校验 → 应用；失败回滚上一份 `config.yaml` |
| **Telegram** | 告警与管理命令（需 Token + Chat ID） |
| **多机** | 登记远程面板、健康检查、节点镜像同步、合并订阅；按名称选择本机节点推送到远程（默认禁用） |
| **运维** | 核心启停/更新、Geo、面板 SSL/ACME、备份、**Cloudflare WARP** 一键注册（WireGuard/MASQUE YAML） |

---

### 补充说明

- [用户限制：IP · 订阅拉取](../users-limits.md)
- [路由与策略组](../routing.md)
- [Cloudflare WARP](../warp.md)
- [按节点流量与倍率](../node-traffic.md)
- [批量应用节点证书](../batch-certificate.md)
- [订阅格式与 target](../subscription-formats.md)

---

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

安装后更新：

```bash
sudo 3m-ui update
# sudo 3m-ui update v1.0.2
```

默认面板：`http://服务器IP:8080/` — 用户 `admin`，安装时打印一次性随机密码（**首次登录请立即修改**）。

### AI 提示词安装

把提示词复制到 ChatGPT / Claude / Cursor 等助手，并说明你的系统（如 Ubuntu 22.04）与是否已有 root SSH。助手应只协助执行官方脚本，不要改写成不明来源命令。完整提示词及使用说明见 [AI 提示词安装文档](../ai-install-prompt.md)。

无需手动复制大段文本，用 `curl` 直接把纯文本提示词加载到剪贴板再粘贴：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | wl-copy       # Linux Wayland
```

提示词只引导使用下方官方一键脚本，所有下载均校验 `SHA256SUMS`：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

---

## 源码编译

```bash
cd frontend && npm install && npm run build
cp -r dist ../backend/cmd/server/web/dist
cd ../backend && CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

多架构静态 Linux 二进制见 [Releases](https://github.com/kazeyukiro/3m-ui/releases)。

---

## 致谢

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 核心引擎与 Listener 模型
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 示例
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — 面板交互参考
- [Gin](https://github.com/gin-gonic/gin) / [GORM](https://github.com/go-gorm/gorm) / [React](https://github.com/facebook/react) / [Ant Design](https://github.com/ant-design/ant-design)
- [Lucide](https://github.com/lucide-icons/lucide) — 图标（`lucide-react`）
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo 分流配置与精简规则思路参考
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo 规则与策略组方案参考
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — 规则集与 mihomo 分流模板参考
- [Zustand](https://github.com/pmndrs/zustand) / [golang-jwt/jwt](https://github.com/golang-jwt/jwt)
- Go、Node.js 与开源社区

贡献者：[Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## 许可证

[Eclipse Public License 2.0](../../LICENSE)

第三方组件见 [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)。
