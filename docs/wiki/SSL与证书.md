# SSL 与证书

## 面板 HTTPS（ACME）

**系统设置 → 面板 SSL**：

- 使用 Let’s Encrypt（autocert）或手动证书
- 配置域名、缓存目录、可选手动 cert/key 路径
- **保存后需要重启面板进程** 才能生效

HTTP-01 需要 80 端口可达（或按你的部署方式完成校验）。

## 节点 TLS

Listener 上可填写证书路径，或在反代（Nginx / Caddy）上终止 TLS，后端只跑明文端口。

**系统设置** 中可有证书申请向导（生成 certbot 命令示例），在服务器上执行后把路径填回 Listener。

## 订阅与反代

推荐：

```text
客户端 ──HTTPS──► 反代 ──► 面板 :8080
```

订阅 URL 使用对外域名；节点 **Public Host** 填客户端应连接的域名或 IP。
