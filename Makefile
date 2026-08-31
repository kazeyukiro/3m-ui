APP_NAME := 3m-ui
BACKEND_DIR := backend
FRONTEND_DIR := frontend
DIST_DIR := dist
VERSION ?= v1.0.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)

# Pure-Go static build flags (portable across glibc/musl Linux).
STATIC_TAGS := sqlite_modernc
STATIC_ENV := CGO_ENABLED=0

.PHONY: build frontend-assets build-linux build-linux-static release clean

# Build the web UI and copy the complete Vite output into the directory used
# by go:embed. Every binary target depends on this so standalone Linux builds
# never ship with stale or missing frontend assets.
frontend-assets:
	cd $(FRONTEND_DIR) && npm install && npm run build
	rm -rf $(BACKEND_DIR)/cmd/server/web/dist
	mkdir -p $(BACKEND_DIR)/cmd/server/web
	cp -R $(FRONTEND_DIR)/dist $(BACKEND_DIR)/cmd/server/web/dist

# Local / host build (uses default sqlite driver; may require CGO on some setups).
build: frontend-assets
	mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME) ./cmd/server

# All official Linux release targets: pure static, max portability.
build-linux: frontend-assets
	mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=amd64 GOAMD64=v1 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=arm64 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-arm64 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=arm GOARM=7 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-armv7 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=arm GOARM=6 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-armv6 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=386 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-386 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=riscv64 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-riscv64 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=loong64 go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-loong64 ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=ppc64le go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-ppc64le ./cmd/server
	cd $(BACKEND_DIR) && $(STATIC_ENV) GOOS=linux GOARCH=s390x go build -tags $(STATIC_TAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-s390x ./cmd/server

# Alias: static is the only official release mode.
build-linux-static: build-linux

release: clean build-linux
	cp scripts/install.sh scripts/update.sh scripts/uninstall.sh scripts/3m-ui.sh scripts/3m-ui $(DIST_DIR)/

clean:
	rm -rf $(DIST_DIR)
