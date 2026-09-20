# Per-node traffic & multiplier

## Concepts

| Term | Meaning |
|------|---------|
| **Raw** | Real upload/download on that listener for the user |
| **Multiplier** | Listener field `traffic_multiplier` (default **1**, range about 0.01–100) |
| **Billed** | `raw × multiplier` — this is what is added to the user’s `traffic_used` quota |

Example: multiplier `1.5`, 100 MiB raw → **150 MiB** counted against the user limit.

## Where to configure

- **Nodes (Listeners)** → edit node → **Traffic multiplier**
- **Users** → node-traffic action (chart icon) → per-node table (raw + billed)

## API

```http
GET /api/v1/users/{id}/node-traffic
```

Response items include `upload_bytes`, `download_bytes`, `traffic_used` (raw), `multiplier`, `billed_upload`, `billed_download`, `billed_used`.

Listener create/update accepts `traffic_multiplier` (omit or `≤0` → treated as `1`).

## Notes

- Attribution uses Mihomo connection `inboundName` ↔ listener **name**.
- Resetting user traffic (single, batch, monthly, or cycle reset) clears per-node rows for that scope.
- Historical traffic from before this feature only exists as the user total; new usage is split by node going forward.
