# Cloudflare WARP

3m-ui can **register a Cloudflare WARP account** and return a Mihomo outbound YAML fragment (WireGuard or MASQUE). This matches the behaviour of tag **v1.3.6**.

## Where

**Settings → Network (or the section with Geo files) → Cloudflare WARP**

1. Choose **WireGuard** or **MASQUE**
2. Click **Register WARP**
3. YAML is copied (when clipboard allows) and shown in a dialog

API:

- `POST /api/v1/system/templates/warp/register?mode=wireguard|masque|both`
- `POST /api/v1/system/templates/warp` — build YAML from operator-supplied keys (advanced)

## What it is / is not

| Yes | No |
|-----|-----|
| One-click register + YAML for **client** or manual Mihomo outbounds | Automatic inject into Routing templates |
| Panel needs **outbound HTTPS** to Cloudflare | Server-side “all traffic via WARP” exit path |

Paste the YAML into a Mihomo client config, or use the fields under a `proxies:` entry of type `wireguard` / `masque` (MASQUE needs a Mihomo build that supports it).

## 中文

**设置 → 网络（含 Geo 的那一栏）→ Cloudflare WARP**：一键注册，生成 WireGuard / MASQUE 出站 YAML（与 v1.3.6 相同）。

- 需要面板能访问 Cloudflare
- **不会**自动写入路由页模板，也**不会**把面板服务端改成 WARP 出口
- 把 YAML 用在客户端或自行合并到 Mihomo 配置即可

接口：`POST /api/v1/system/templates/warp/register?mode=wireguard|masque|both`
