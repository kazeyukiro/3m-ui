# NAT 与面板端口

适用于家宽 / 内网穿透 / 仅映射部分端口的 VPS。核心需求：**自定义面板端口**、**正确公网地址**，让订阅与管理面板在 NAT 后仍可访问。

## 快速改端口

### 安装时指定

```bash
PANEL_PORT=8443 curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

| 环境变量 | 作用 | 默认 |
|----------|------|------|
| `PANEL_PORT` / `THREE_M_UI_PORT` | 面板监听端口 | `8080` |
| `PANEL_LISTEN` / `THREE_M_UI_LISTEN` | 绑定地址 | 空（全部网卡，双栈友好） |
| `PUBLIC_URL` / `THREE_M_UI_PUBLIC_URL` | 公网面板 URL（订阅/外链） | 空 |

示例（仅本机反代访问 + 固定公网 URL）：

```bash
PANEL_PORT=8080 PANEL_LISTEN=127.0.0.1 PUBLIC_URL=https://panel.example.com \
  curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

### 已安装：改配置文件

编辑 `/etc/3m-ui/config.yaml`：

```yaml
server:
  port: 8443
  listen: ""                              # 空 = 所有接口
  public_url: "https://your-domain:8443"  # NAT 后订阅用
  mode: release
```

重启生效：

```bash
systemctl restart 3m-ui
# OpenRC: rc-service 3m-ui restart
```

### 面板 UI

**系统设置 → 面板 / NAT**：

- **面板端口**：写入 `config.yaml` 的 `server.port`
- **监听地址**：`server.listen`（`127.0.0.1` 适合仅反代）
- **面板公网 URL**：`server.public_url`

保存后提示 **重启服务** 后端口/监听才会切换；`public_url` 会立即用于部分链接生成逻辑（进程内配置已更新）。

### API

```http
GET  /api/v1/system/panel-server
PUT  /api/v1/system/panel-server
Content-Type: application/json

{"port":8443,"listen":"","public_url":"https://panel.example.com:8443"}
```

需管理员 JWT。返回中 `restart_required: true` 表示需重启进程。

### 环境变量覆盖（一次启动）

进程启动时读取（优先级高于 YAML 中的同名项）：

- `THREE_M_UI_PORT` / `PANEL_PORT`
- `THREE_M_UI_LISTEN` / `PANEL_LISTEN`
- `THREE_M_UI_PUBLIC_URL` / `PUBLIC_URL`

systemd 示例（`/etc/systemd/system/3m-ui.service.d/override.conf`）：

```ini
[Service]
Environment=THREE_M_UI_PORT=8443
Environment=THREE_M_UI_PUBLIC_URL=https://panel.example.com:8443
```

```bash
systemctl daemon-reload && systemctl restart 3m-ui
```

## NAT 网络建议

1. **面板端口**：路由器/云安全组把公网端口映射到内网机的 `server.port`。
2. **节点端口**：每个 Listener 的入站端口同样需要映射；与面板端口无关。
3. **Public Host / Public Port**：在节点或系统设置的访问配置中填写**客户端真实连接**的地址/端口（公网 IP 或域名、映射后的端口）。
4. **public_url**：填写浏览器能打开的面板完整 URL，订阅信息页、部分外链会参考它。
5. **仅反代暴露**：`listen: "127.0.0.1"`，由 Nginx/Caddy 提供 443；面板不必对公网直接开放。

## 与节点分享地址的区别

| 配置 | 用途 |
|------|------|
| `server.port` / `listen` | 面板 HTTP(S) 进程绑定 |
| `server.public_url` | 面板自身的对外 URL |
| 节点 Public Host/Port、access profile | 客户端连 **代理协议** 的地址 |
| 订阅链接里的 host | 来自 Public Host / 请求 Host / public_url 等解析逻辑 |

改面板端口 **不会** 自动改节点入站端口。

## 故障排查

| 现象 | 处理 |
|------|------|
| 改端口后打不开 | 是否已 `restart`；防火墙/安全组是否放行新端口 |
| 订阅里 host 仍是内网 IP | 设置节点 Public Host 或 `public_url` / 访问配置 |
| 端口被占用 | `ss -lntp \| grep <port>`，换端口或停掉占用进程 |
| 仅 IPv6 / 仅 IPv4 | 调整 `server.listen`（如 `127.0.0.1`、`::1` 或留空双栈） |

相关文档：[[面板配置]] · [[系统设置]] · [[订阅]] · [[安装与升级]]
