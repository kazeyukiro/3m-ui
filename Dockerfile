# Multi-stage build: pure-Go static 3m-ui binary with embedded Ant Design frontend.
# Compatible with glibc and musl hosts (scratch/alpine/distroless).

# TODO(supply-chain, H-3): pin base images to immutable digests for reproducible
# builds. Floating tags (node:22-alpine, golang:1.25-alpine, alpine:3.21) are
# mutable — only @sha256:<digest> guarantees byte-identical rebuilds. Compute
# digests with `docker pull <image> && docker inspect --format='{{.RepoDigests}}' <image>`
# and replace each `FROM` line below with `<image>@sha256:<digest>`.
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/dist ./cmd/server/web/dist
ENV CGO_ENABLED=0
RUN go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o /out/3m-ui ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
# Run as a non-root user (H-2 / C-4). The panel binary does not need root in
# container mode. Mihomo TUN mode requires CAP_NET_ADMIN and is expected to run
# on the host (or in a privileged sidecar) — not inside this panel container.
RUN addgroup -S -g 10001 3m-ui && adduser -S -D -H -u 10001 -G 3m-ui 3m-ui
WORKDIR /app
COPY --from=backend /out/3m-ui /usr/local/bin/3m-ui
# Pre-create the volume mount points with correct ownership so named volumes
# (docker-compose) inherit UID 10001 on first creation. Bind-mounted host dirs
# must also be `chown 10001:10001` on the host by the operator.
RUN mkdir -p /etc/3m-ui /var/lib/3m-ui /var/log/3m-ui && \
    chown -R 3m-ui:3m-ui /app /usr/local/bin/3m-ui /etc/3m-ui /var/lib/3m-ui /var/log/3m-ui
# Default paths match the installer layout.
ENV THREE_M_UI_CONFIG=/etc/3m-ui/config.yaml
VOLUME ["/etc/3m-ui", "/var/lib/3m-ui", "/var/log/3m-ui"]
EXPOSE 8080
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/3m-ui"]
