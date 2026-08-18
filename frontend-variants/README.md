# 3M-UI Frontend Editions

3M-UI keeps the backend, API contract and configuration model shared while offering independent frontend editions.

## Editions

| Edition | UI stack | Workflow |
|---|---|---|
| Ant Design | React + Ant Design | `release-ant.yml` |
| Material | React + MUI | `release-mui.yml` |
| Mantine | React + Mantine | `release-mantine.yml` |
| shadcn-style | React + Tailwind CSS + shadcn-style primitives | `release-shadcn.yml` |

The Ant Design frontend remains the reference implementation. Alternative editions use the same API and session contract, so they can be packaged with the same Go backend without changing backend behavior.

## Build locally

```bash
cd frontend-variants/mui
npm install
npm run build
```

Replace `mui` with `mantine` or `shadcn` for the other editions.

## Release

Each edition has its own GitHub Actions workflow and produces separate Linux `amd64`, `arm64`, and `armv7` archives. A workflow is started manually with a release tag.

The selected frontend is copied into `backend/cmd/server/web/dist` only inside the CI workspace before the Go binary is built. CI does not commit generated frontend assets back to `main`.

## Shared contract

`frontend-variants/shared/api.ts` contains the common authentication/session, dashboard, listener, user, cluster, config and Mihomo process API layer. UI code must not duplicate backend semantics.

## Important

Alternative editions intentionally start with the high-value operational surfaces: login, dashboard, Mihomo control, listeners, users, cluster and generated YAML. The Ant Design edition remains the full-featured reference UI while the alternative editions are expanded independently without coupling the backend to a UI library.
