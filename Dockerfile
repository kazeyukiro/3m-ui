# Build frontend and Go on the builder's native architecture; only the final
# runtime stage needs emulation when producing the other supported platform.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /src/backend
RUN apk add --no-cache ca-certificates
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/dist ./cmd/server/web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOAMD64=v1 \
    go build -tags sqlite_modernc -trimpath \
    -ldflags="-s -w -X github.com/kazeyukiro/3m-ui/backend/internal/buildinfo.Version=$VERSION -X github.com/kazeyukiro/3m-ui/backend/internal/buildinfo.GitCommit=$GIT_COMMIT -X github.com/kazeyukiro/3m-ui/backend/internal/buildinfo.BuildTime=$BUILD_TIME" \
    -o /out/3m-ui ./cmd/server

FROM --platform=$BUILDPLATFORM alpine:3.21 AS mihomo
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl
COPY distribution/mihomo.env /src/distribution/mihomo.env
COPY scripts/download-mihomo.sh /src/scripts/download-mihomo.sh
RUN sh /src/scripts/download-mihomo.sh "$TARGETARCH" /out

FROM alpine:3.21
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG REPOSITORY=kazeyukiro/3m-ui
LABEL org.opencontainers.image.title="3m-ui" \
      org.opencontainers.image.source="https://github.com/${REPOSITORY}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}"
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 3m-ui && adduser -S -D -H -u 10001 -G 3m-ui 3m-ui && \
    mkdir -p /etc/3m-ui /var/lib/3m-ui /var/log/3m-ui /usr/local/lib/3m-ui && \
    chown -R 10001:10001 /etc/3m-ui /var/lib/3m-ui /var/log/3m-ui
COPY --from=backend /out/3m-ui /usr/local/bin/3m-ui
COPY --from=mihomo /out/mihomo /usr/local/lib/3m-ui/mihomo
COPY --from=mihomo /out/MIHOMO_VERSION /usr/local/lib/3m-ui/MIHOMO_VERSION
COPY distribution/MIHOMO_LICENSE /usr/local/share/licenses/mihomo/LICENSE
COPY LICENSE /usr/local/share/licenses/3m-ui/LICENSE
RUN printf 'Mihomo source and build scripts: https://github.com/MetaCubeX/mihomo/tree/%s\n' \
    "$(cat /usr/local/lib/3m-ui/MIHOMO_VERSION)" > /usr/local/share/licenses/mihomo/SOURCE
# Image files remain root-owned. The bundled core is the initial fallback;
# administrator-selected versions persist under /var/lib/3m-ui/mihomo/cores.
# Panel image updates preserve that selection through the existing data volume.
ENV THREE_M_UI_CONFIG=/etc/3m-ui/config.yaml \
    THREE_M_UI_CONTAINER=1
VOLUME ["/etc/3m-ui", "/var/lib/3m-ui", "/var/log/3m-ui"]
EXPOSE 8080
USER 10001:10001
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/3m-ui", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/3m-ui"]
