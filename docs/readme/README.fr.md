# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Langues / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · **Français** · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Panneau web d’administration pour serveur Mihomo**

> **La stabilité d’abord :** l’installation et la mise à jour n’utilisent par défaut que les releases officielles. Les préversions doivent être choisies explicitement. Les paquets d’installation complets et les images Docker contiennent une version fixe de Mihomo ; voir [Installation, mise à niveau et restauration](https://3m-ui.top/guide/install).

Léger et auto-hébergé, pour gérer les Listener, les utilisateurs, les abonnements et l’état d’exécution de [Mihomo](https://github.com/MetaCubeX/mihomo) sous Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Documentation : [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Site web : [https://3m-ui.top/](https://3m-ui.top/)

---

## Fonctionnalités

| Catégorie | Capacités |
|------|------|
| **Nœuds** | Listener pilotés par un registre de protocoles : VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, etc. ; Reality, TLS auto-signé, certificats par lot en un clic, **multiplicateur de trafic**, validation des champs de transport mutuellement exclusifs |
| **Utilisateurs** | **Création en un clic** (nom d’utilisateur/mot de passe/UUID aléatoires, liaison facultative à tous les nœuds), liaison de nœuds, quota de trafic, **comptage par nœud** (facturation = réel × multiplicateur du nœud), expiration, limites IP / tirage d’abonnement, départ à la première utilisation, renouvellement/réinitialisation cyclique, étiquettes de groupe, fusion d’abonnements externes, opérations par lot, jeton d’abonnement |
| **Abonnements** | Détection automatique par UA de Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` toujours en Base64), sing-box JSON ; `?target=` facultatif ; page d’abonnement HTML ; les liens de partage TUIC/HY2 incluent SNI/`allow_insecure` et d’autres paramètres client |
| **Configuration** | Générer → valider → appliquer, étapes séparées ; retour à la précédente `config.yaml` en cas d’échec |
| **Telegram** | Alertes et commandes d’administration (Token + Chat ID requis) |
| **Multi-serveurs** | Enregistrement de panneaux distants, contrôle de santé, synchronisation des miroirs de nœuds, fusion des abonnements ; **push des nœuds locaux par nom** vers les distants (désactivé par défaut, renommable/port modifiable) |
| **Sécurité** | JWT, changement de mot de passe à la première connexion, double authentification TOTP, **GitHub OAuth** facultatif, préfixe de chemin web_path, chiffrement des identifiants, CORS |
| **Exploitation** | Démarrage/arrêt/mise à jour du cœur, Geo, SSL/ACME du panneau (y compris les caractères génériques DNS-01), sauvegarde, enregistrement **Cloudflare WARP** en un clic (WireGuard/MASQUE YAML) |

### Précisions (limites utilisateur / certificats / abonnements)

- [Limites utilisateur : IP · tirages d’abonnement](https://3m-ui.top/guide/users-limits)
- [Trafic par nœud et multiplicateur](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME du panneau (HTTP-01, caractère générique DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [Appliquer des certificats de nœuds par lot](https://3m-ui.top/guide/batch-certificate)
- [Formats d’abonnement et target](https://3m-ui.top/guide/subscription-formats)
- [Nœuds multi-serveurs / push](https://3m-ui.top/guide/cluster)
- [Routage (abonnement client + sortie serveur)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend : **React + Ant Design + Lucide Icons** (`frontend/`), intégré après compilation dans un unique binaire Go.

Artefacts de release : **liaison purement statique** (`CGO_ENABLED=0` + modernc SQLite), aucun `libsqlite3` système requis, compatible glibc / musl.

Paquet d’installation complet / Docker : `linux/amd64` · `arm64`. Le binaire de panneau autonome avancé prend également en charge `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` ; un noyau compatible doit être fourni par vos soins.

---

## Installation rapide

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

L’installation complète prend en charge Linux amd64 / arm64 et installe automatiquement le panneau, le Mihomo correspondant et le service système. La configuration initiale et le mot de passe administrateur sont générés de façon uniforme par le programme ; si la configuration existe déjà, les valeurs sont conservées. Pour les autres architectures, utilisez le binaire autonome et un noyau préparé par vos soins, voir [instructions d’installation détaillées](https://3m-ui.top/guide/install).

Après l’installation :

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Mise à jour :

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Désinstallation :

```bash
sudo 3m-ui uninstall
```

### Installation avec un prompt IA

Copiez le prompt dans un assistant tel que ChatGPT / Claude / Cursor et indiquez votre système (par ex. Ubuntu 22.04) ainsi que la présence ou non d’un accès SSH root. L’assistant doit uniquement aider à exécuter le script officiel et ne pas le réécrire en commandes de provenance inconnue. Le prompt complet et les notes d’utilisation se trouvent dans la [documentation du prompt d’installation IA](../ai-install-prompt.md).

Pas besoin de copier manuellement un long texte : chargez le prompt en texte brut directement dans le presse-papiers avec `curl`, puis collez-le :

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Le prompt ne fait qu’orienter vers le script officiel en une ligne ci-dessous ; tous les téléchargements sont vérifiés avec `SHA256SUMS` :

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Le canal de test / préversion (facultatif, non par défaut) est décrit dans la [documentation d’installation](../installation.md#ai-提示词安装).

---

## Installation Docker

Sur un VPS Linux, téléchargez le fichier `docker-compose.yml` de la release et exécutez dans le même répertoire :

```bash
docker compose up -d
docker compose logs 3m-ui
```

L’image est `ghcr.io/kazeyukiro/3m-ui:latest` et contient le panneau ainsi qu’une version fixe de Mihomo ; `latest` ne suit que les versions stables. La configuration, les clés aléatoires et l’administrateur sont initialisés automatiquement, le premier mot de passe n’est affiché qu’une seule fois au premier démarrage. Le réseau host est utilisé par défaut ; après l’ajout de nœuds, ouvrez les ports TCP/UDP réellement utilisés. Un fichier `docker-compose.bridge.yml` est également fourni, ainsi que des indications sur le version pinning, la configuration des certificats et la restauration persistante, voir [documentation d’installation](https://3m-ui.top/guide/install).

---

## Compiler depuis les sources

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Un binaire **Mihomo** compatible doit être installé sur la machine et son chemin indiqué dans la configuration.

---

## Points de configuration

Les données et la configuration par défaut se trouvent approximativement aux emplacements suivants :

| Chemin | Usage |
|------|------|
| `/etc/3m-ui/config.yaml` | Configuration du panneau |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Certificats auto-signés des nœuds (**à sauvegarder avec la base de données**) |
| `/var/lib/3m-ui/mihomo/` | Répertoire de travail de Mihomo et `config.yaml` |

Utilisez des valeurs aléatoires distinctes (≥ 32 octets) pour les clés JWT et les identifiants ; n’utilisez pas les exemples de la documentation.

Port de panneau personnalisé : `3m-ui config port <port>` ou modifiez la configuration puis redémarrez le service.

Les nouveaux nœuds reçoivent par défaut un port d’écoute aléatoire compris entre 10000 et 60000 (échantillonnage léger évitant les ports des nœuds déjà chargés ; les plages décroissantes sont prises en charge) ; il peut être modifié manuellement ou via le bouton « 随机端口 ».

---

## Schéma d’architecture

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

## Recommandations de sauvegarde

Sauvegardez au minimum :

1. `3m-ui.db`
2. `listener-certs/`
3. Le `config.yaml` du panneau et le répertoire de configuration de Mihomo

Lors d’une simple mise à jour du binaire, les certificats sont autant que possible hydrate depuis le disque ; si le répertoire est perdu, ils sont de nouveau auto-signés et les clients doivent retirer l’abonnement.

---

## Remerciements

Un remerciement particulier aux projets open source et aux communautés suivants :

- [Mihomo](https://github.com/MetaCubeX/mihomo) — moteur proxy principal et modèle de configuration des Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — demande, déploiement et renouvellement de certificats dans le menu SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — exemples de configuration de Listener et référence des protocoles
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — référence pour l’interaction du panneau et les approches d’exploitation
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP backend
- [GORM](https://github.com/go-gorm/gorm) — ORM de base de données
- [React](https://github.com/facebook/react) — base du frontend
- [Ant Design](https://github.com/ant-design/ant-design) — composants d’interface
- [Lucide](https://github.com/lucide-icons/lucide) — icônes (`lucide-react`, adapté via `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — référence pour la configuration de routage Mihomo et les règles allégées
- [echs-top/proxy](https://github.com/echs-top/proxy) — référence pour les règles Mihomo et les schémas de groupes de proxies
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — référence pour les ensembles de règles et les modèles de routage mihomo
- [Zustand](https://github.com/pmndrs/zustand) — gestion d’état frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js et toute la communauté open source

Contributeurs : [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Licence

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

À partir du commit `226c460`, ce projet est placé sous la **GNU General Public License v3.0**. Les commits antérieurs et les versions déjà publiées restent soumis à l’**Eclipse Public License 2.0 (EPL-2.0)**, cette licence ne pouvant être révoquée.

[GNU General Public License v3.0](../../LICENSE)

Composants tiers : [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## English & other languages

Traductions complètes (18 langues) : voir les liens de langue en haut de page ou ouvrez [README.en.md](./README.en.md).
