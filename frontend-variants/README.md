# 3M-UI Frontend Editions

Three independent UI kits sharing **the same** API, i18n, stores, and listener serialization as Ant Design (`frontend/`).

## Editions

| Edition | Path | Stack |
|---|---|---|
| Ant Design (reference) | `frontend/` | antd + `@ant-design/icons` |
| Material | `frontend-variants/mui` | MUI 9 + `@mui/icons-material` |
| Mantine | `frontend-variants/mantine` | Mantine 9 + `@tabler/icons-react` |
| shadcn-style | `frontend-variants/shadcn` | Tailwind 4 + `lucide-react` |

## Alignment with Ant Design

### Routes (identical)
`/login` `/change-password` `/` `/listeners` `/users` `/traffic` `/cluster` `/routing` `/core` `/logs` `/config` `/settings`

### Listeners (aligned)
- CRUD, reload, URI export
- **Full protocol form** via shared `listenerFormSchema` (shadowsocks / snell / vmess / vless / trojan / hysteria2 / tuic / shadowquic / anytls / mieru / sudoku / trusttunnel)
- Transport (TCP / WS / gRPC / XHTTP), Security (none / TLS / Reality)
- Wrappers: simple-obfs, shadow-tls, res-tls, JLS, mux, kcp-tun, mkcp, mekya, realm, JLS upstream
- VLESS encryption / decryption + flow (xtls-rprx-vision)
- Access Profile (public host/port/SNI/fingerprint/ALPN)
- Capability-driven fields + `capabilityFormToConfig`
- Templates, instantiate, clone, version history, diff, rollback
- Batch enable / disable

### Users (aligned)
- CRUD, traffic limit (GB), IP limit, expire
- Bind listeners, subscription URL/token, reset traffic, copy sub token

### Other pages
Dashboard / Core / Logs / Config (Monaco) / Traffic / Cluster / Routing / Settings (i18n, theme, password, backup, Telegram, OpenAPI)

### Shared logic (`frontend-variants/shared/`)
- `api/*` — full axios client
- `logic/listenerConfig.ts` — `configToFormValues` / `formValuesToConfig`
- `logic/capabilityForm.ts` — capability serialization
- `logic/listenerFormSchema.ts` — declarative form sections (Ant field coverage)
- `i18n` / `stores` / `utils`

Vite alias: `@shared/...`

## Build

```bash
cd frontend-variants/mui   # or mantine / shadcn
npm install && npm run build
```

`base: './'` for `go:embed`. Dev server proxies `/api` → `:8080`.
