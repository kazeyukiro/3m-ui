# GitHub OAuth（面板登录）

3m-ui 支持用 **GitHub OAuth** 登录管理面板（可选）。密码登录与 TOTP 仍然可用。

## 配置步骤

1. 在 GitHub：**Settings → Developer settings → OAuth Apps → New OAuth App**
2. **Authorization callback URL** 设为：

   `https://<面板公网域名>/api/v1/auth/oauth/github/callback`

   主机与协议需与面板 **public_url** 一致。

3. 面板 **设置 → 安全与备份 → GitHub OAuth**：
   - 填入 Client ID / Client Secret
   - **允许的 GitHub 用户名**（逗号分隔）：首次登录时用于绑定本地管理员
   - 启用并保存

4. 登录页会出现「使用 GitHub 登录」。

## 绑定规则

- 已绑定过的 `github_id`：直接登录对应用户
- 在允许名单中且存在同名本地用户：绑定并登录
- 在允许名单中且系统仅有一个 `admin`：绑定到该管理员
- 否则拒绝

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/auth/oauth/github` | 公开：是否启用 |
| GET | `/api/v1/auth/oauth/github/start` | 跳转 GitHub 授权 |
| GET | `/api/v1/auth/oauth/github/callback` | 回调，再跳转 `/login?oauth_token=...` |
| GET/PUT | `/api/v1/auth/oauth/github/settings` | 管理员读写配置 |

面板服务器需要能访问 `github.com` 与 `api.github.com`。
