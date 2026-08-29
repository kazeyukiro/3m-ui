# 3M-UI 使用文档

**3M-UI** 是面向自托管 VPS / Linux 服务器的轻量 Mihomo Web 管理面板。

| 项目 | 说明 |
|------|------|
| 仓库 | https://github.com/kazeyukiro/3m-ui |
| 核心 | Mihomo（Clash Meta） |
| 前端 | React + Ant Design（官方唯一前端） |
| 发布 | 纯 Go 静态二进制，多架构 Linux |

## 文档目录

1. [[安装与升级]]
2. [[快速开始]]
3. [[面板配置]]
3.1. [[NAT与面板端口]]
4. [[节点-Listeners]]
5. [[用户与流量]]
6. [[订阅]]
7. [[核心管理]]
8. [[多机节点-Cluster]]
9. [[Telegram-Bot]]
10. [[SSL与证书]]
11. [[系统设置]]
12. [[备份与恢复]]
13. [[API与认证]]
14. [[故障排查]]
15. [[安全建议]]

## 功能概览

- Mihomo 启动 / 停止 / 重启与状态监控
- Listener（入站）全生命周期管理与配置校验
- 多协议：VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC 等
- 代理用户：流量配额、到期、IP 限制、批量操作
- 多格式订阅：Mihomo YAML、Clash、V2Ray Base64、Sing-box JSON、HTML 信息页
- 面板 ACME / 一键 SSL、自定义订阅页模板
- Telegram 通知（登录、流量、到期、CPU、日摘要）与简易 Bot 指令
- 多机远程面板登记、健康检查与受限远程管控
- IPv6 / 双栈监听与分享链接
- JWT 认证、凭据加密、配置失败回滚

## 支持架构

`amd64` · `arm64` · `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`
