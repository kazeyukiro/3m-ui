# Multi-stage build: pure-Go static 3m-ui binary with embedded Ant Design frontend.
# Compatible with glibc and musl hosts (scratch/alpine/distroless).

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
WORKDIR /app
COPY --from=backend /out/3m-ui /usr/local/bin/3m-ui
# Default paths match the installer layout.
ENV THREE_M_UI_CONFIG=/etc/3m-ui/config.yaml
VOLUME ["/etc/3m-ui", "/var/lib/3m-ui", "/var/log/3m-ui"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/3m-ui"]
