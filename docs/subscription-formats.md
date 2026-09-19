# Subscription formats

Public token URL (path may use `web_path` / custom sub base):

`/api/v1/client/sub/{token}` or configured subscription base + token.

## `?target=`

| target | Body |
|--------|------|
| *(empty / clash / default)* | Mihomo / Clash Meta YAML |
| `v2ray` / `base64` | **Always** standard Base64 of newline-separated share links (`vless://`, `vmess://`, `tuic://`, …). Independent of HTML page “encrypt URI list”. |
| `uri` / `raw` | Same links; may be plaintext when encrypt is off |
| `singbox` / `sing-box` | sing-box JSON outbounds |

UA auto-detection may choose Clash vs v2ray-style when `target` is omitted.

## TUIC / Hysteria2 share links

Exported URIs include client-oriented params (`sni`, `alpn`, `congestion_control`, `allow_insecure` / `allowInsecure`, etc.). Classic v2rayNG may list `tuic://` but not dial QUIC; prefer Clash Meta / NekoBox / Hiddify for those protocols.
