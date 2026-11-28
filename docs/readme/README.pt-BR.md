# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Idiomas / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · **Português (Brasil)** · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Painel web de gerenciamento do servidor Mihomo**

> **Estabilidade primeiro:** a instalação e a atualização usam por padrão apenas releases oficiais. As pré-releases precisam ser escolhidas explicitamente. Os pacotes de instalação completos e as imagens Docker incluem uma versão fixa do Mihomo; consulte [Instalação, atualização e restauração](https://3m-ui.top/guide/install).

Leve e auto-hospedado, para gerenciar Listener, usuários, assinaturas e o estado de execução do [Mihomo](https://github.com/MetaCubeX/mihomo) no Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Documentação: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Site: [https://3m-ui.top/](https://3m-ui.top/)

---

## Recursos

| Categoria | Capacidades |
|------|------|
| **Nós** | Listener baseados em um registro de protocolos: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC etc.; Reality, TLS autoassinado, certificados em lote com um clique, **multiplicador de tráfego**, validação de campos de transporte mutuamente exclusivos |
| **Usuários** | **Criação com um clique** (nome de usuário/senha/UUID aleatórios, com opção de vincular todos os nós), vinculação de nós, limite de tráfego, **medição por nó** (cobrança = real × multiplicador do nó), expiração, limites de IP / obtenção de assinatura, contagem a partir do primeiro uso, renovação/reset por ciclo, tags de grupo, fusão de assinaturas externas, operações em lote, Token de assinatura |
| **Assinaturas** | Detecção automática por UA de Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` sempre Base64), sing-box JSON; `?target=` opcional; página de assinatura HTML; os links de compartilhamento TUIC/HY2 incluem SNI/`allow_insecure` e outros parâmetros de cliente |
| **Configuração** | Gerar → validar → aplicar, etapas separadas; reversão para a `config.yaml` anterior em caso de falha |
| **Telegram** | Alertas e comandos de administração (requer Token + Chat ID) |
| **Multi-servidor** | Registro de painéis remotos, verificação de integridade, sincronização de espelhos de nós, fusão de assinaturas; **push de nós locais por nome** para os remotos (desativado por padrão, podendo renomear/alterar porta) |
| **Segurança** | JWT, troca de senha no primeiro login, autenticação de dois fatores TOTP, **GitHub OAuth** opcional, prefixo de caminho web_path, credenciais criptografadas, CORS |
| **Operação** | Iniciar/parar/atualizar o núcleo, Geo, SSL/ACME do painel (incluindo curingas DNS-01), backup, registro do **Cloudflare WARP** com um clique (WireGuard/MASQUE YAML) |

### Observações adicionais (limites de usuário / certificados / assinaturas)

- [Limites de usuário: IP · obtenções de assinatura](https://3m-ui.top/guide/users-limits)
- [Tráfego por nó e multiplicador](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME do painel (HTTP-01, curinga DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [Aplicar certificados de nós em lote](https://3m-ui.top/guide/batch-certificate)
- [Formatos de assinatura e target](https://3m-ui.top/guide/subscription-formats)
- [Nós multi-servidor / push](https://3m-ui.top/guide/cluster)
- [Roteamento (assinatura de cliente + saída de servidor)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`), incorporado após a compilação em um único binário Go.

Artefatos de release: **vinculação puramente estática** (`CGO_ENABLED=0` + modernc SQLite), sem necessidade do `libsqlite3` do sistema, compatível com glibc / musl.

Pacote de instalação completo / Docker: `linux/amd64` · `arm64`. O binário de painel autônomo avançado também suporta `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`; é necessário preparar por conta própria um kernel compatível.

---

## Instalação rápida

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

A instalação completa suporta Linux amd64 / arm64 e instala automaticamente o painel, o Mihomo correspondente e o serviço do sistema. A configuração inicial e a senha de administrador são geradas de forma padronizada pelo programa; se a configuração já existir, os valores são mantidos. Para outras arquiteturas, é possível usar o binário autônomo e um kernel preparado por conta própria, consulte [instruções detalhadas de instalação](https://3m-ui.top/guide/install).

Após a instalação:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Atualização:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Desinstalação:

```bash
sudo 3m-ui uninstall
```

### Instalação com prompt de IA

Copie o prompt para um assistente como ChatGPT / Claude / Cursor e informe seu sistema (por exemplo, Ubuntu 22.04) e se já possui SSH root. O assistente deve apenas ajudar a executar o script oficial, sem reescrevê-lo em comandos de origem desconhecida. O prompt completo e as instruções de uso estão na [documentação do prompt de instalação com IA](../ai-install-prompt.md).

Não é necessário copiar manualmente um texto longo: carregue o prompt em texto puro diretamente para a área de transferência com `curl` e cole:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

O prompt apenas orienta o uso do script oficial de uma linha abaixo; todos os downloads são verificados com `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

O canal de teste / pré-release (opcional, não padrão) está na [documentação de instalação](../installation.md#ai-提示词安装).

---

## Instalação via Docker

Em um VPS Linux, baixe o arquivo `docker-compose.yml` da release e execute no mesmo diretório:

```bash
docker compose up -d
docker compose logs 3m-ui
```

A imagem é `ghcr.io/kazeyukiro/3m-ui:latest` e contém o painel e uma versão fixa do Mihomo; `latest` acompanha apenas as versões estáveis. Configuração, chaves aleatórias e o administrador são inicializados automaticamente; a senha inicial é exibida apenas uma vez, no primeiro início. Por padrão é usada a rede host; após adicionar nós, libere as portas TCP/UDP efetivamente utilizadas. Também são fornecidos `docker-compose.bridge.yml` e instruções sobre fixação de versão, configuração de certificados e restauração persistente, consulte [documentação de instalação](https://3m-ui.top/guide/install).

---

## Compilar a partir do código-fonte

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

É necessário ter instalado localmente um binário **Mihomo** compatível e apontar seu caminho na configuração.

---

## Pontos de configuração

Os dados e a configuração padrão ficam aproximadamente em:

| Caminho | Finalidade |
|------|------|
| `/etc/3m-ui/config.yaml` | Configuração do painel |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Certificados autoassinados dos nós (**faça backup junto com o banco de dados**) |
| `/var/lib/3m-ui/mihomo/` | Diretório de trabalho do Mihomo e `config.yaml` |

Use valores aleatórios próprios (≥ 32 bytes) para as chaves de JWT e credenciais; não use os exemplos da documentação.

Porta personalizada do painel: `3m-ui config port <port>` ou edite a configuração e reinicie o serviço.

Novos nós preenchem por padrão uma porta de escuta aleatória entre 10000 e 60000 (amostragem leve que evita as portas de nós já carregados; há suporte a intervalos decrescentes); é possível alterar manualmente ou clicar em «随机端口».

---

## Diagrama de arquitetura

```text
  Browser (Ant Design + Lucide)
        │  REST + JWT
        ▼
  Gin API  ── Telegram Bot
        │
   Service layer (users / listeners / sub / cluster / …)
        │
   SQLite · certstore · visual config
        │
        ▼
     Mihomo (-t 校验 → 应用 / 回滚)
```

---

## Recomendações de backup

Faça backup de, no mínimo:

1. `3m-ui.db`
2. `listener-certs/`
3. O `config.yaml` do painel e o diretório de configuração do Mihomo

Ao atualizar apenas o binário, os certificados são hydrate do disco sempre que possível; se o diretório for perdido, eles serão autoassinados novamente e os clientes precisarão obter a assinatura outra vez.

---

## Agradecimentos

Agradecimento especial aos seguintes projetos de código aberto e comunidades:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — motor de proxy principal e modelo de configuração de Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — emissão, implantação e renovação de certificados no menu SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — exemplos de configuração de Listener e referência de protocolos
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — referência de interação do painel e ideias de operação
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP do backend
- [GORM](https://github.com/go-gorm/gorm) — ORM de banco de dados
- [React](https://github.com/facebook/react) — base do frontend
- [Ant Design](https://github.com/ant-design/ant-design) — componentes de UI
- [Lucide](https://github.com/lucide-icons/lucide) — ícones (`lucide-react`, adaptado por `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — referência de configuração de roteamento do Mihomo e ideias de regras enxutas
- [echs-top/proxy](https://github.com/echs-top/proxy) — referência de regras do Mihomo e esquemas de grupos de proxies
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — referência de conjuntos de regras e modelos de roteamento mihomo
- [Zustand](https://github.com/pmndrs/zustand) — gerenciamento de estado do frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js e toda a comunidade de código aberto

Contribuidores: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Licença

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

A partir do commit `226c460`, este projeto adota a **GNU General Public License v3.0**. Os commits anteriores e as versões já publicadas permanecem sob a **Eclipse Public License 2.0 (EPL-2.0)**, licença que não pode ser revogada.

[GNU General Public License v3.0](../../LICENSE)

Componentes de terceiros: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## English & other languages

Traduções completas (18 idiomas): veja os links de idioma no topo ou abra [README.en.md](./README.en.md).
