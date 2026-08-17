APP_NAME := 3m-ui
BACKEND_DIR := backend
FRONTEND_DIR := frontend
DIST_DIR := dist
VERSION ?= v0.1.0-rc.3
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)

.PHONY: build frontend-assets build-linux build-linux-static release clean

# Build the web UI and copy the complete Vite output into the directory used
# by go:embed. Every binary target depends on this so standalone Linux builds
# never ship with stale or missing frontend assets.
frontend-assets:
	cd $(FRONTEND_DIR) && npm install && npm run build
	rm -rf $(BACKEND_DIR)/cmd/server/web/dist
	mkdir -p $(BACKEND_DIR)/cmd/server/web
	cp -R $(FRONTEND_DIR)/dist $(BACKEND_DIR)/cmd/server/web/dist

build: frontend-assets
	mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME) ./cmd/server

build-linux: frontend-assets
	mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-arm64 ./cmd/server
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-armv7 ./cmd/server

build-linux-static: frontend-assets
	mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-amd64-static ./cmd/server
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-arm64-static ./cmd/server
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/$(APP_NAME)-linux-armv7-static ./cmd/server

release: clean build build-linux build-linux-static
	cp scripts/install.sh scripts/update.sh scripts/uninstall.sh $(DIST_DIR)/

clean:
	rm -rf $(DIST_DIR)
