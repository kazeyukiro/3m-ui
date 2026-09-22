# Multi-node (cluster)

Register other **3m-ui** panels and operate them from one place.

## Remote panel entry

| Field | Meaning |
|-------|---------|
| Name | Display label |
| Panel URL | Base URL of the remote panel (e.g. `https://panel.example.com`, no API path) |
| API token / Login | JWT used for remote admin API; use **Login** on the row if operations return 401 |
| Enabled | Whether the entry is active |

Health check can work with limited access; most operations need a valid token.

## Sync nodes

**Sync** pulls remote listeners into local **mirrored node** records (share URI / client YAML) so this panel can merge them into user subscriptions.

## Push local node

**Push** clones a **local** listener onto the remote panel.

1. Open **Cluster** → **Push** on a remote row.
2. Pick a local node **by name** (protocol and port are shown). You do **not** type a database ID.
3. Optionally set remote name / port (defaults: `<name>-remote` and the local port).
4. Review the dry-run payload, then confirm.

The remote copy is always created **disabled** so it will not bind the port until you enable it on the remote panel.

API:

```http
POST /api/v1/cluster/:remoteId/push-node
{ "local_node_id": 12, "dry_run": false, "new_name": "optional", "new_port": "optional" }
```

The UI resolves `local_node_id` from the selected local listener.

## Security notes

- Remote `base_url` is validated to reduce SSRF (private/link-local targets are restricted).
- Prefer HTTPS for remote panel URLs.
- Tokens are full admin credentials on the remote — treat them like passwords.
