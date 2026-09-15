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
- Go 1.25+

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


## Future Features

The architecture should allow adding:

- Traffic statistics
- Multiple Mihomo instances
- Multi-user management
- Docker deployment
- Cluster management
