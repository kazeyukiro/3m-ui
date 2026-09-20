# User limits: IP and subscription pulls

Panel fields on each **ProxyUser** (Users page / API).

## IP limit (`ip_limit`)

| Value | Meaning |
|-------|---------|
| `0` | Unlimited concurrent source IPs |
| `N ≥ 1` | At most **N** distinct client source IPs online at once |

**How it works:** the traffic collector polls Mihomo connections about every **5 seconds**. Connections that cannot be attributed to a user are ignored. When a user exceeds `N` source IPs, excess IPs’ connections are closed (not blocked at handshake time).

## Subscription pull limit (`sub_pull_limit`)

| Value | Meaning |
|-------|---------|
| `0` | Unlimited successful subscription fetches |
| `N ≥ 1` | At most **N** successful pulls per **rolling 24 hours** |

Exceeded → **HTTP 429** with `Retry-After` and JSON `error: subscription pull limit reached (per 24h)`.

HTML subscription pages and client downloads both count.

## Related APIs

- User CRUD: `ip_limit`, `sub_pull_limit` on create/update
- Subscription: public `/api/v1/client/sub/{token}` (and aliases)

## Related

- [Per-node traffic & multiplier](node-traffic.md)
