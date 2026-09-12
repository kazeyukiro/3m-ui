# 安装、升级与恢复

3m-ui 提供两种完整安装方式：Linux 一键安装，以及预构建 Docker 镜像。两者均包含面板和固定版本的 Mihomo，使用相同的首次初始化逻辑。默认安装、更新及 Docker `latest` 只跟随稳定版。

## Linux 一键安装

完整安装包支持 Linux amd64、arm64，要求 root、systemd 或 OpenRC、可访问 GitHub 的网络。不需要 Go、Node 或系统 libsqlite3。安装过程自动补齐下载和解压工具。

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

需要先检查脚本或指定参数时，下载到本地再执行：

```bash
curl -fsSLo install.sh https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh
sudo env PANEL_PORT=8080 sh install.sh --yes
```

环境变量必须传给执行脚本的 `sh`，放在管道左侧的 `curl` 前面不会传递给右侧的安装程序。

首次安装默认使用 `8080`。放行面板和各节点实际使用的端口后，访问 `http://SERVER_IP:8080/`。安装时会显示管理员 `admin` 及随机初始密码，首次登录必须修改；遗失密码可执行 `sudo 3m-ui reset-admin`，生成新密码并使旧登录会话失效。密码不会写入配置文件或安装结果文件。

配置文件已存在时会保留，包括现有账号、密码、端口和密钥。重复运行安装程序用于更新程序文件，不会重置管理员。修改已安装面板的端口使用 `sudo 3m-ui config port 9000`。

从旧版安装脚本迁移时，先执行上面的最新安装命令一次，以安装新的快照与恢复流程；旧版 `3m-ui update` 不具备本文描述的完整备份保证。


### AI 提示词安装

把提示词复制到 ChatGPT / Claude / Cursor 等助手，并说明你的系统（如 Ubuntu 22.04）与是否已有 root SSH。助手应只协助执行官方脚本，不要改写成不明来源命令。完整提示词及使用说明见 [AI 提示词安装文档](ai-install-prompt.md)。

无需手动复制大段文本，用 `curl` 直接把纯文本提示词加载到剪贴板再粘贴：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.zh.txt | wl-copy       # Linux Wayland
```

提示词只引导使用上方官方一键脚本，所有下载均校验 `SHA256SUMS`。

测试 / 预发布通道（可选，非默认）：

```text
在基础要求上，若用户明确要预发布/测试版，再使用：
sudo env THREE_M_UI_CHANNEL=pre sh -c 'curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sh'
或已安装后：sudo 3m-ui update pre
默认仍应安装稳定版。
```

完整预发布提示词可追加到主提示词末尾，详见 [AI 提示词安装文档](ai-install-prompt.md#测试--预发布通道可选非默认)。


### HTTPS 与对外访问

首次登录改密后，在「系统设置」配置面板 HTTPS：可以申请域名证书，也可以使用已有证书。已有 Nginx / Caddy 的用户可设置面板只监听 `127.0.0.1`，由反向代理提供 HTTPS。域名不是安装前置要求；HTTP 端口本身不代表已启用 HTTPS。

配置证书时，将自管证书放在 `/etc/3m-ui/` 或 `/var/lib/3m-ui/`，以便与数据库、密钥一起备份。域名解析、HTTP-01 验证所需端口、防火墙与云安全组需要与所选择的访问方式匹配。

### 版本与来源

`v1.2.3` 等标签是不可变的正式版本，`v1.3.0-rc.1` 是显式选择的候选版本。下列版本号仅为格式示例，请换成 Releases 中实际存在的版本。

```bash
sudo sh install.sh v1.2.3
sudo 3m-ui update v1.2.3

# 主动加入滚动测试通道；此选择会保留至后续更新。
sudo env THREE_M_UI_CHANNEL=pre 3m-ui update

# 返回稳定版更新通道。
sudo env THREE_M_UI_CHANNEL=stable 3m-ui update
```

没有稳定版可用时会明确失败，不会自动改装 `pre`。默认不会自动升级；执行更新命令时才下载目标版本。

发布自己的 fork 时，需要显式选定发行来源，例如：

```bash
sudo env THREE_M_UI_REPO=YOUR_OWNER/3m-ui sh install.sh
```

此来源会保存在安装目录，后续更新沿用它。Docker 使用相同发行仓库的镜像，例如 `ghcr.io/YOUR_OWNER/3m-ui:v1.2.3`。不能假定从 fork 下载原始脚本后，默认下载地址也会自动改变。

所有发行下载均校验 `SHA256SUMS`，校验失败就停止。已安装 `cosign` 的用户可设置 `THREE_M_UI_VERIFY_COSIGN=1`，额外要求工作流签名验证。

### 自定义内核及高级架构

完整安装默认使用 `/usr/local/lib/3m-ui/mihomo`，不自动复用全局 `/usr/local/bin/mihomo`。既有配置指定的内核路径会保留。

`--no-mihomo` 只安装 / 更新面板，适用于已有外部内核，或只提供独立面板二进制的其他架构。新装时可设置 `THREE_M_UI_MIHOMO_BINARY=/absolute/path/to/mihomo`。自定义核心需要自行验证兼容性并备份；完整发行包只承诺清单指定的配套版本。

### 管理与备份

```bash
sudo 3m-ui status
sudo 3m-ui version
sudo 3m-ui logs
sudo 3m-ui restart
sudo 3m-ui backup
sudo 3m-ui update
```

`backup` 会短暂停服，在 `/var/lib/3m-ui/backups/` 创建完整快照，然后恢复原先的运行状态。快照包含面板程序、配套核心、管理脚本、服务定义、配置、加密密钥、数据库及其辅助文件、Mihomo 数据和证书；历史备份不会递归收入新快照。

更新先完成下载和校验，再停服建立一致快照，替换程序并执行初始化与 HTTP/HTTPS 健康检查。更新失败会尝试恢复整份快照，包括迁移前的数据库和密钥。恢复失败时会输出可用快照位置。

```bash
# 换成实际创建的绝对路径；命令会先为当前状态再做一份备份。
sudo 3m-ui restore /var/lib/3m-ui/backups/ACTUAL-SNAPSHOT.tar.gz
```

快照恢复要求原安装路径一致。恢复其他版本时使用对应快照，不能只换旧二进制并假定数据库能够降级。请将重要快照复制到其他机器，定期实际演练恢复。

自动快照覆盖本安装的配置与数据目录。若已配置的数据库、核心配置或证书实际位于其他共享目录，脚本会在停服前报出路径并停止，避免声称已经完成备份。先把相关存储迁入受管理目录，或自行完成相应的备份和手动更新。外部核心二进制不会被替换。

证书、核心和 ACME 目录内的文件符号链接也会检查实际目标；嵌套目录符号链接和失效链接不支持自动快照，需要先整理为受管理目录内的实际文件和目录。

```bash
sudo 3m-ui uninstall          # 保留配置、数据、密钥和证书
sudo 3m-ui uninstall --purge  # 明确确认后彻底删除本安装的持久化内容
```

## Docker

使用本发行仓库 Releases 中的 `docker-compose.yml`，固定 Compose 所在目录，避免改变项目名后创建另一组空卷。

```bash
docker compose up -d
docker compose logs 3m-ui
```

镜像包含面板、配套 Mihomo，首次启动自动生成配置与密钥，并在日志中输出一次随机初始密码。只有密码哈希写入数据库；首次登录改密后可按自己的日志保留策略轮转首次启动日志。配置、数据和日志使用命名卷，卷权限由镜像准备。重启与容器重建不会再次生成账号或密钥。

镜像以非 root 用户运行。普通监听默认不需要 `privileged` 或 `NET_ADMIN`。低于 1024 的监听端口如受宿主机限制，可以使用高端口或为确有需要的部署添加 `NET_BIND_SERVICE`；TUN 等功能的权限需要自行配置。

### 网络与版本

默认 Compose 使用 Linux host 网络。新建监听会直接绑定宿主机端口，不需要重新创建容器，仍需处理端口冲突、防火墙和安全组。

需要 bridge 网络时，独立使用另一个模板：

```bash
docker compose -f docker-compose.bridge.yml up -d
```

bridge 模板仅把面板映射到 `127.0.0.1:8080`；公网访问需要反向代理或调整绑定。每个节点需要对应的 TCP/UDP 映射，模板包含端口范围示例。不要将 host 和 bridge 模板叠加使用。

建议在 Compose 同目录的 `.env` 中固定正式版本，例如：

```dotenv
THREE_M_UI_IMAGE=ghcr.io/kazeyukiro/3m-ui:v1.2.3
```

将示例换成实际发布标签。默认 `latest` 只跟随稳定版，`pre` 只用于主动参与测试。fork 的镜像仓库与发行仓库一致。

### Docker 升级、备份与恢复

升级通过替换镜像完成，容器内不运行宿主机安装脚本。升级前先拉取新镜像，再停服备份持久化目录：

```bash
docker compose pull
docker compose stop
mkdir -p backups
chmod 700 backups
docker compose run --rm --no-deps --user 0 --entrypoint sh \
  -v "$PWD/backups:/backup" 3m-ui -c \
  'umask 077; tar -czf /backup/before-upgrade.tar.gz /etc/3m-ui /var/lib/3m-ui'
docker compose up -d
docker compose ps
```

每次使用不同备份文件名；示例中的 `before-upgrade.tar.gz` 会被覆盖。证书和核心配置应存放在这两个持久化目录内。记录备份对应的镜像标签或 digest；需要恢复时停服，将 `.env` 改回该镜像，再恢复整份快照：

```bash
docker compose stop
docker compose run --rm --no-deps --user 0 --entrypoint sh \
  -v "$PWD/backups:/backup:ro" 3m-ui -c \
  'tar -tzf /backup/before-upgrade.tar.gz >/dev/null &&
   find /etc/3m-ui /var/lib/3m-ui -mindepth 1 -maxdepth 1 -exec rm -rf {} \; &&
   tar -xzf /backup/before-upgrade.tar.gz -C /'
docker compose up -d
```

恢复前还应备份当前卷。上述恢复命令先验证备份可读取，再清空这两个受管理目录的现有内容，避免将旧的 WAL 等辅助文件混入恢复结果。`docker compose down` 保留卷，`docker compose down -v` 会删除持久化卷。

遗失管理员密码时：

```bash
docker compose exec 3m-ui /usr/local/bin/3m-ui reset-admin
```

## 维护发行版本

固定 Mihomo 版本、架构资产名和 SHA256 在 `distribution/mihomo.env`。调整此文件意味着更新配套内核，必须重新执行安装与核心兼容验证。原生完整包和 Docker 构建共用这份清单。

正式 / 候选版本工作流按标签构建，并拒绝覆盖已发布版本。滚动 `pre` 是独立通道。镜像发布到 `ghcr.io/<当前仓库所有者>/<当前仓库名>`；仓库管理员首次发布时需要确保 package 可见性适合公众拉取。

发布验证应涵盖两种架构、首次初始化、真实核心运行、登录改密、创建节点、实际代理连接、更新保留数据和快照恢复。仅编译成功不能证明一台全新服务器可以安装。


## Subscription path and port

Optional `server.sub_path` (e.g. `/sub`) and `server.sub_port` in `/etc/3m-ui/config.yaml`, or env `THREE_M_UI_SUB_PATH` / `THREE_M_UI_SUB_PORT`. Legacy `/api/v1/client/sub/:token` remains. Set `public_url` to the client-facing base URL.

## Independent core updates

The Core page supports manual updates and rollback of official stable and Pre Mihomo
versions on Linux amd64/arm64. Selected versions persist in the existing data
volume across panel upgrades. See [core updates](core-updates.md) for validation,
recovery, and deployment integration details.

## Frontend compression and caching

The panel precompresses embedded text assets with gzip once at startup and serves
compressed responses when the client accepts them. Hashed JavaScript and CSS
assets use `Cache-Control: public, max-age=31536000, immutable`; HTML and assets
without content hashes use `no-cache` with ETags so new deployments are detected.
Identity and gzip responses have separate validators and use `Vary: Accept-Encoding`.
No reverse-proxy compression setting or writable asset directory is required.
Missing `/assets/` files return an uncached 404 rather than the SPA HTML fallback.
