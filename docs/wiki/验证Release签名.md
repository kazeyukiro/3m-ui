# 验证 Release 签名

3M-UI 的每个 Release 资产都经过两层校验，确保你下载和运行的代码来自本项目且未被篡改：

1. **完整性**（默认强制）：`SHA256SUMS` 防止二进制损坏或中间人篡改。
2. **真实性**可选但推荐：`SHA256SUMS.pem` + `SHA256SUMS.sig` 由 **Sigstore cosign keyless OIDC** 签发，证明 `SHA256SUMS` 本身由本项目 `release.yml` workflow 在对应 tag 上生成。

两层一起构成端到端供应链验证：即便 CDN 或 release 被劫持，攻击者也无法同时伪造 `SHA256SUMS` 与 cosign 签名（签名密钥是 GitHub Actions 的临时 OIDC token，不存储在仓库里）。

## 资产清单

每个 Release（v1.0.0 起）附带：

| 资产 | 用途 |
|------|------|
| `3m-ui-linux-<arch>` | 各架构纯 Go 静态二进制 |
| `3m-ui-linux-<arch>.tar.gz` | 打包版本 |
| `install.sh` / `update.sh` / `uninstall.sh` / `3m-ui.sh` / `3m-ui` | 安装器与入口脚本 |
| `SHA256SUMS` | 上述全部资产的 SHA-256 校验和 |
| `SHA256SUMS.pem` | cosign 签名证书（含 OIDC 身份） |
| `SHA256SUMS.sig` | cosign 签名（base64） |

## 一、验证二进制完整性（默认强制）

### 方式 A：安装器自动验证（推荐）

`install.sh` / `update.sh` 默认 fail-closed 校验 `SHA256SUMS`：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

若 `SHA256SUMS` 缺失或校验和不匹配，安装会**中止**。GitHub 不可用时如需绕过，显式设置 `THREE_M_UI_INSECURE=1`（**不推荐**，会打印醒目警告）。

### 方式 B：手动验证

下载二进制与 `SHA256SUMS` 后：

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

`--ignore-missing` 允许你只下载了部分资产（例如只下了 `3m-ui-linux-amd64`）。

## 二、验证签名真实性（推荐）

### 前置：安装 cosign

```bash
# Linux x86_64 示例；其它架构见 https://github.com/sigstore/cosign/releases
curl -fsSL -o /usr/local/bin/cosign \
  https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
chmod +x /usr/local/bin/cosign
cosign version
```

### 方式 A：安装器自动验证

安装/升级时启用 cosign 校验（OPT-IN）：

```bash
THREE_M_UI_VERIFY_COSIGN=1 \
  curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | bash
```

安装器会：

1. 检查 `cosign` 是否在 `PATH` 中；不在则打印警告并跳过（不中止安装）。
2. 下载 `SHA256SUMS` / `SHA256SUMS.pem` / `SHA256SUMS.sig`。
3. 依次尝试两种 OIDC 身份验证：
   - `https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/tags/<tag>`（tag push 触发的 release）
   - `https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/heads/main`（workflow_dispatch 回填的 release）
4. 任一匹配即通过；都失败则**中止**（仅 `THREE_M_UI_INSECURE=1` 可绕过，不推荐）。

升级同理：

```bash
THREE_M_UI_VERIFY_COSIGN=1 \
  curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/update.sh | bash
```

### 方式 B：手动验证

下载 `SHA256SUMS`、`SHA256SUMS.pem`、`SHA256SUMS.sig` 三个文件后：

```bash
# 1. 先验证二进制完整性
sha256sum -c SHA256SUMS --ignore-missing

# 2. 验证 SHA256SUMS 的真实性
cosign verify-blob \
  --certificate SHA256SUMS.pem \
  --signature SHA256SUMS.sig \
  --certificate-identity 'https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/tags/<tag>' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
```

- 把 `<tag>` 替换为实际 release tag（例如 `v1.0.0`）。
- **若该 release 是通过 `workflow_dispatch` 回填签名的**（如 v1.0.0），OIDC 身份为 `@refs/heads/main` 而非 `@refs/tags/<tag>`。手动验证时改用：
  ```bash
  --certificate-identity 'https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/heads/main'
  ```
- 验证成功输出：`Verified OK`。

### 方式 C：查看证书身份（可选）

想确认证书 OIDC 身份确实绑定到本仓库：

```bash
# cosign 的 .pem 是 base64 编码的 DER，需先解码
base64 -d SHA256SUMS.pem | openssl x509 -noout -text | grep -A1 "URI:"
```

应看到形如：

```
URI:https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/tags/v1.0.0
```

或回填的：

```
URI:https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/heads/main
```

## 三、完整端到端示例

以 v1.0.0、x86_64 为例：

```bash
set -e
TAG=v1.0.0
ARCH=linux-amd64

# 1. 下载资产
curl -fLO https://github.com/kazeyukiro/3m-ui/releases/download/${TAG}/3m-ui-${ARCH}
curl -fLO https://github.com/kazeyukiro/3m-ui/releases/download/${TAG}/SHA256SUMS
curl -fLO https://github.com/kazeyukiro/3m-ui/releases/download/${TAG}/SHA256SUMS.pem
curl -fLO https://github.com/kazeyukiro/3m-ui/releases/download/${TAG}/SHA256SUMS.sig

# 2. 验证完整性
sha256sum -c SHA256SUMS --ignore-missing

# 3. 验证真实性（v1.0.0 是回填签名，用 refs/heads/main）
cosign verify-blob \
  --certificate SHA256SUMS.pem \
  --signature SHA256SUMS.sig \
  --certificate-identity 'https://github.com/kazeyukiro/3m-ui/.github/workflows/release.yml@refs/heads/main' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS

# 4. 赋予执行权限并使用
chmod +x 3m-ui-${ARCH}
./3m-ui-${ARCH} --version
```

## 常见问题

### Q：旧版本 release 没有签名资产怎么办？

v1.0.0 之前的 release 没有 `SHA256SUMS.pem` / `SHA256SUMS.sig`。安装器会检测到缺失并打印警告继续（不影响 `SHA256SUMS` 的 fail-closed 校验）。建议升级到 v1.0.0+。

### Q：`THREE_M_UI_INSECURE=1` 何时使用？

仅当 GitHub Release 临时不可用、你又必须紧急安装/升级时。该环境变量会同时跳过 `SHA256SUMS` 校验与 cosign 签名校验，并打印醒目警告。**生产环境不应使用**。

### Q：cosign keyless 签名的密钥存放在哪？

不存放任何长期密钥。每次 release workflow 运行时，GitHub 向 Sigstore Fulcio 出示 OIDC token 证明"我是 `kazeyukiro/3m-ui` 的 `release.yml` workflow"，Fulcio 签发短时证书，cosign 用该证书签名 `SHA256SUMS`。验证方只需信任 Sigstore 根 CA 与 GitHub OIDC issuer，无需信任任何项目维护者的长期密钥。

### Q：如何验证 cosign 本身的供应链？

cosign 是 [Sigstore](https://github.com/sigstore/cosign) 项目，自身也用 Sigstore 签名，且其 Releases 经过 SLSA-3 验证。从官方 Release 下载即可。

## 相关

- [[安装与升级]]
- [[安全建议]]
- [[面板配置]]
