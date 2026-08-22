# 多机节点（Cluster）

用于从本机面板 **登记其它 3M-UI 实例**，做健康检查与受限远程管控。

## 登记远程面板

填写：

| 字段 | 说明 |
|------|------|
| 名称 | 显示名 |
| Base URL | 如 `https://panel.example.com:8080`（仅 http/https） |
| API Token | 远程面板管理员登录后的 **JWT Bearer Token** |
| 启用 | 关闭后不探测、不代理 |
| 备注 | 可选 |

Token 不会在列表接口中明文返回，仅显示是否已配置。

## 健康检查

- 单机：探测 `GET {base}/api/v1/health`
- 全部检查：对所有已启用远程面板探测并写入 `last_status` / `last_error` / `last_check_at`

状态：`up` / `down` / `error`。

## 远程管控

在权限与网络允许时，可：

- 拉取远程 Dashboard JSON
- 列表远程用户
- 列表 / 创建 / 删除远程 Listener
- 远程 Start / Stop / Restart Mihomo

通用代理接口仅允许白名单路径前缀，例如：

```text
/api/v1/health
/api/v1/dashboard
/api/v1/nodes
/api/v1/listeners
/api/v1/users
/api/v1/mihomo
/api/v1/system
/api/v1/traffic
```

禁止路径穿越与任意 URL，降低开放代理风险。

## 使用注意

- 远程 Token 权限等同管理员，泄露即控制远程面板
- 建议专用低权限网络、HTTPS、定期轮换 Token
- 本机与远程版本差异可能导致部分 API 字段不兼容
