# Cloudflare WARP

3m-ui can **register a Cloudflare WARP account** and return a Mihomo outbound YAML fragment (WireGuard or MASQUE).

## Where

**Settings → Network (Geo section) → Cloudflare WARP**

1. Choose **WireGuard** or **MASQUE**
2. Click **Register WARP**
3. YAML is shown (and copied when clipboard allows)

API:

- `POST /api/v1/system/templates/warp/register?mode=wireguard|masque|both`
- `POST /api/v1/system/templates/warp` — build YAML from operator-supplied keys (advanced)

## What it is / is not

| Yes | No |
|-----|-----|
| One-click register + YAML fragment | Automatic unlock of Netflix / ChatGPT / etc. |
| Use as **client** proxy or **server egress** outbound | Guaranteed streaming/AI region unlock (depends on Cloudflare edge) |
| Panel needs **outbound HTTPS** to Cloudflare | Auto-detect unlock status and rewrite routing without confirmation |

## Using WARP on the server (egress)

1. Register WARP and copy the `proxies:` entry (name is often `WARP` / `WARP-Masque`).
2. Open **Routing → Server egress**.
3. Add the proxy (or merge YAML into `server-routing` proxies via API), then rules such as:

```text
DOMAIN-SUFFIX,openai.com,WARP
DOMAIN-SUFFIX,chatgpt.com,WARP
MATCH,DIRECT
```

4. **Save → Generate & apply**.

There is **no** fully automatic “probe AI/streaming then always wire WARP” path: unlock results change by IP/edge and site policy; false positives would hijack egress. Manual or one-click **register + optional rules** is the supported model.

## 中文

**设置 → 网络 → Cloudflare WARP**：一键注册，生成 WireGuard / MASQUE 出站 YAML。

- 需要面板能访问 Cloudflare  
- **不会**自动检测流媒体/AI 解锁，也**不会**在无确认时改写全局出口  
- 用作客户端节点，或放到 **路由规则 → 服务端出站**（保存并应用）  

流媒体/AI 是否解锁取决于 WARP 出口质量，**不保证**。
