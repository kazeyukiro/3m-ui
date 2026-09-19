# User limits: IP, HWID, subscription pulls

Panel fields on each **ProxyUser** (Users page / API).

## IP limit (`ip_limit`)

| Value | Meaning |
|-------|---------|
| `0` | Unlimited concurrent source IPs |
| `N ≥ 1` | At most **N** distinct client source IPs online at once |

**How it works:** the traffic collector polls Mihomo connections about every **5 seconds**. Connections that cannot be attributed to a user are ignored. When a user exceeds `N` source IPs, excess IPs’ connections are closed (not blocked at handshake time).

## HWID limit (`hwid_limit`)

Compatible with Happ / Remnawave-style headers: `x-hwid`, `x-device-os`, `x-ver-os`, `x-device-model`.

| Value | Meaning |
|-------|---------|
| `0` | No device cap (devices still recorded when `x-hwid` is sent; DB errors never block the subscription) |
| `N ≥ 1` | At most **N** devices; **subscription requests without a valid `x-hwid` are rejected (403)** |

Device list: `GET /api/v1/users/{id}/hwid-devices`. Delete one or clear all via the Users UI / DELETE APIs.

## Subscription pull limit (`sub_pull_limit`)

| Value | Meaning |
|-------|---------|
| `0` | Unlimited successful subscription fetches |
| `N ≥ 1` | At most **N** successful pulls per **rolling 24 hours** |

Exceeded → **HTTP 429** with `Retry-After` and JSON `error: subscription pull limit reached (per 24h)`.

HTML subscription pages and client downloads both count.

## Related APIs

- User CRUD: `ip_limit`, `hwid_limit`, `sub_pull_limit` on create/update
- Subscription: public `/api/v1/client/sub/{token}` (and aliases)
