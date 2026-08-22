# API 与认证

## 基址

```text
http(s)://<host>:<port>/api/v1
```

健康检查（可无需登录，视版本而定）：

```text
GET /api/v1/health
```

## 登录

```http
POST /api/v1/auth/login
Content-Type: application/json

{"username":"admin","password":"..."}
```

响应中的 JWT 用于后续请求：

```http
Authorization: Bearer <token>
```

## 常用资源（需认证）

| 前缀 | 用途 |
|------|------|
| `/users` | 用户 CRUD、批量、清理耗尽 |
| `/nodes` · `/listeners` | 入站管理 |
| `/mihomo/*` | 核心启停 |
| `/system/*` | 备份、Geo、SSL、订阅页等 |
| `/telegram/*` | Bot 设置 |
| `/cluster/*` | 多机节点 |
| `/traffic` | 流量 |

OpenAPI：`GET /api/v1/openapi.yaml`（若已启用）。

## 公共订阅

订阅接口使用 **sub_token**，不走管理员 JWT。详见 [[订阅]]。

## 多机远程

Cluster 代理使用本机管理员 JWT 调用本机 `/cluster`，由本机再持远程 JWT 访问远端。远程 Token 妥善保管。
