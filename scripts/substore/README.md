# Sub-Store 转换内核集成

3m-ui 的订阅**序列化输出**中，除 Clash/Mihomo YAML 与 sing-box JSON 保留原生实现外，其余目标格式（Loon / Surge / Quantumult X / v2ray Base64 / URI / Stash / Shadowrocket / Surfboard / Egern 等）均由内嵌的 **Sub-Store 转换内核**生成，以保证与官方 Sub-Store 输出逐字节一致。

## 为什么 Clash YAML 和 sing-box 不切

- **Clash/Mihomo YAML**：这是 3m-ui 的原生主格式，包含面板自己的 `clientSubscriptionDocument`（策略组、视觉配置等），Sub-Store 的通用 producer 无法复现这些定制内容。
- **sing-box**：3m-ui 做了 SFA/SFI 客户端版本适配（`http_clients` / `route.default_http_client`，sing-box 1.14+ 才支持），Sub-Store 的通用 sing-box producer 不含这些字段，切换会破坏客户端兼容。

除此之外，所有格式都走 Sub-Store。

## 架构

```
订阅请求 ?target=loon|surge|qx|v2ray|uri|...
        │
        ▼
converter.GenerateUserViaSubStore
        │  复用 userBoundListeners 生成 share URI 订阅原文
        ▼
engine.Pool (sync.Pool 并发安全)
        │  goja.Runtime（纯 Go，无 CGO）
        ▼
bundle/substore.js  ←  Sub-Store parse + produce 核心（自包含纯 JS）
        │
        ▼
目标格式字符串
```

- **goja** 是纯 Go 的 ECMAScript 引擎，不需要 CGO，因此 9 个发布架构全部可用。
- **bundle** 是平台无关的 JS，随二进制 `go:embed` 嵌入，无外部文件依赖。
- bundle 已经 stub 掉所有 Node 侧依赖（fs / database / gist / dns / geo / sync / app），只保留纯转换路径；不依赖 Buffer / TextEncoder / process 全局。

## 上游版本

- 上游项目：https://github.com/sub-store-org/Sub-Store
- 集成时的上游分支：`master`
- **集成日期**：见本文件 git 提交历史
- bundle 由上游 `backend/src/core/proxy-utils/{parsers,processors,producers}` 等打包而成。

## 重建 bundle

当上游 Sub-Store 更新协议适配时，按以下步骤重建：

```sh
# 1. 拉取指定版本的 Sub-Store 后端源码（需要 Node.js 22 + pnpm）
git clone --depth 1 https://github.com/sub-store-org/Sub-Store.git /path/to/sub-store
cd /path/to/sub-store/backend && pnpm install

# 2. 用本项目脚本打包（把上游 backend 目录作为参数传入）
cd /path/to/3m-ui
node scripts/substore/build-bundle.js /path/to/sub-store/backend

# 3. 验证：golden 对照测试 + 回归测试
cd backend
go test ./internal/substore/engine/ -v     # 与官方 golden 数据逐字节对比
go test ./...                               # 全量回归
```

打包脚本做的事：
- 用 esbuild 把 `core/proxy-utils/index.js` 打成 IIFE 单文件（`--target=es2016`）。
- alias `@/` → `src/`，`buffer` → 纯 JS polyfill。
- 用 browser 条件解析 `fastestsmallesttextencoderdecoder`，得到纯 JS 版 TextEncoder/TextDecoder。
- stub 掉转换路径用不到的 Node 侧模块（database/sync/gist/fs/dns/geo/app + 所有 Node 内置模块）。
- 末尾追加桥接，暴露全局 `SubStoreCore = { parse, produce }`。

## 对照测试（"真机实测"的代码级等价验证）

`backend/internal/substore/engine/engine_test.go` 中的 golden 数据，来自官方 Sub-Store **2.44.31**（commit `d0bf093`）的 `/api/proxy/parse` 实跑输出，覆盖 9 种协议（SS / VMess / VLESS / Trojan / Hysteria2 / AnyTLS / TUIC 等）× Loon / Surge / QX 三种格式，共 25 行，逐字节比对。

```sh
cd backend && go test ./internal/substore/engine/ -v
```

> 说明：受运行环境限制，无法在 iOS/Mac 真机（Surge/Loon/Quantumult X 客户端）上做端到端实测。此处采用更强的代码级等价验证——以官方服务实跑抓取的输出为 golden，比对内嵌内核的输出，确保零误差。建议维护者在真机上做一次最终验收。

## 多架构验证

```sh
sh scripts/substore/check-crossarch.sh
```

覆盖 amd64 / arm64 / armv7 / armv6 / 386 / riscv64 / loong64 / ppc64le / s390x 全部 9 个发布架构。

## 紧急回退

若内嵌内核出现问题，设置环境变量即可临时回退到旧版原生渲染（仅影响非 Clash/sing-box 格式）：

```
SUB_STORE_CONVERTER=0
```
