# Batch apply TLS certificate to nodes

Apply one certificate + private key to many listeners in a single request (Users → Nodes: select rows → **Apply cert**).

## API

`POST /api/v1/nodes/batch/certificate` (admin JWT)

```json
{
  "ids": [1, 2, 3],
  "certificate": "-----BEGIN CERTIFICATE-----...",
  "private_key": "-----BEGIN PRIVATE KEY-----...",
  "cert_file": "/etc/letsencrypt/live/example.com/fullchain.pem",
  "key_file": "/etc/letsencrypt/live/example.com/privkey.pem",
  "from_panel_ssl": false
}
```

Provide **either** PEM strings, **or** both file paths (allowlisted directories such as `/etc/letsencrypt/`, `/var/lib/3m-ui/`), **or** `from_panel_ssl: true` (panel SSL manual `cert_file` / `key_file`).

Response: `{ "updated": [1, 2], "failed": [{ "id": 3, "name": "...", "error": "..." }] }`.

Writes `certificate` and `private-key` into each listener’s config JSON and schedules a Mihomo reload. Protocols that reject certificate mode (e.g. pure Reality) appear under `failed`.
