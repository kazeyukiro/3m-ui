# 3m-ui Development Guide

## Project Overview

3m-ui is a web management panel for Mihomo Core.

The goal is to provide a simple and powerful VPS management interface based on Mihomo Core.

Main functions:

- Manage Mihomo Core
- Create and manage Mihomo listeners
- Generate Mihomo configuration files
- Manage users
- Generate subscriptions
- Monitor service status
- View logs


## Technology Stack

### Backend

Language:
- Go 1.26+ (1.26.10 or newer; earlier 1.25 builds lack the crypto/x509 fix for CVE-2026-111943)

Framework:
- Gin

Database:
- SQLite (default)
- GORM ORM

Authentication:
- JWT


### Frontend

Framework:
- React 19

Language:
- TypeScript

Build tool:
- Vite

UI:
- Ant Design

State management:
- Zustand or Redux Toolkit

Routing:
- React Router


## Project Architecture


Backend:

backend/

├── cmd/
│   └── server/

├── internal/

│   ├── auth/
│   │   Authentication logic

│   ├── mihomo/
│   │   Mihomo Core management

│   ├── listener/
│   │   Listener management

│   ├── subscription/
│   │   Subscription generation

│   ├── system/
│   │   Linux system operations

│   └── database/
│       Database models and migrations



Frontend:

frontend/

src/

├── api/
│   API requests

├── pages/
│   Main pages

├── components/
│   Reusable components

├── layouts/
│   Page layouts

└── stores/
    State management



## Mihomo Rules

All Mihomo related code MUST be placed inside:

backend/internal/mihomo/


The Mihomo service layer is responsible for:

- Starting Mihomo
- Stopping Mihomo
- Restarting Mihomo
- Checking status
- Reading logs
- Calling Mihomo External Controller API


Do not directly execute Mihomo commands inside API handlers.


Example:

Correct:

API Handler
    |
    ↓
Mihomo Service
    |
    ↓
Mihomo Core


Incorrect:

API Handler
    |
    ↓
Execute mihomo command directly



## Configuration Rules

Mihomo configuration must be generated through templates.

Do not manually concatenate YAML strings.

Configuration flow:

Database

↓

Config Generator

↓

YAML Template

↓

config.yaml

↓

Mihomo Reload



## Database Rules

Use GORM models.

Main tables:


users

- id
- username
- password_hash
- role
- created_at


listeners

- id
- name
- type
- listen
- port
- enabled
- config


listener_users

- id
- listener_id
- username
- password


subscriptions

- id
- user_id
- token
- format
- expire_time


configs

- id
- name
- content


## API Rules

All APIs use:

/api/v1/


Example:

GET:

/api/v1/mihomo/status


POST:

/api/v1/listeners


DELETE:

/api/v1/listeners/:id



## Frontend Rules

Use Ant Design components.

Keep pages separated.

Do not put business logic directly inside components.

Use:

API Layer

↓

State Store

↓

Components



## Security Rules

Never store plaintext passwords.

Use password hashing.

Validate all user input.

Do not expose sensitive system information.

Do not allow arbitrary command execution from Web UI.



## Development Rules

Before implementing a large feature:

1. Explain the design
2. Confirm database changes
3. Confirm API changes
4. Then write code


Keep the project modular.

Avoid unnecessary dependencies.

Prefer simple and maintainable solutions.


## Feature Status

Most originally-planned capabilities are already implemented. The list below
separates what exists today from work that is still open.

### Implemented

- Listener / node management: VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC,
  AnyTLS, Snell, ShadowQUIC; REALITY, self-signed TLS, one-click batch certificates,
  traffic ratio, transport-field mutual-exclusion validation
- User management: one-click create (random user/pass/UUID, optional bind-all),
  per-node metering (usage × node ratio), limits, expiry, grouping, subscription token
- Subscription generation: Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray`),
  sing-box JSON, HTML subscription page
- Config: generate → validate → apply, rollback to previous `config.yaml` on failure
- Telegram alerts & admin commands
- Multi-machine cluster: register remote panels, health check, node mirror sync, push
- Security: JWT, forced password change on first login, TOTP, optional GitHub OAuth,
  `web_path` prefix, credential encryption, CORS
- Ops: core start/stop/update, Geo data, panel SSL/ACME (HTTP-01, DNS-01 wildcard),
  backup, Cloudflare WARP one-click registration
- DNS control UI (#105), Outbound management (#104)
- REALITY target catalog: reviewed candidates in
  `backend/internal/mihomo/realityscan/candidates.json` (see `realityscan/README.md`);
  one-click creation pulls from `auto_select: true` entries only
- i18n across 15+ locales
- Traffic statistics dashboard

### Open / possible future work

- Audit-log viewer UI (backend `panellog` already records events)
- Alert channels beyond Telegram (Webhook / Bark / DingTalk / Discord / email)
- Listener templates / clone for fast same-shape node creation
- One-click full backup/restore (config + users + subscriptions + certificates)
- Traffic trend charts and per-user / per-node Top-N
- Remove deprecated fields (DNS `geosite`, `warp_register` legacy `x/crypto`,
  `mihomo/api` deprecated `Network`, `listener_user` legacy fields) before a Mihomo
  upgrade breaks them
