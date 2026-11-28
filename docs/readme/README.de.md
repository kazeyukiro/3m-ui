# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Sprachen / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · **Deutsch** · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo Server-Web-Verwaltungspanel**

> **Stabil zuerst:** Installation und Aktualisierung verwenden standardmäßig ausschließlich offizielle Releases. Vorabversionen müssen aktiv ausgewählt werden. Vollständige Installationspakete und Docker-Images enthalten eine feste Mihomo-Version; siehe [Installation, Upgrade und Wiederherstellung](https://3m-ui.top/guide/install).

Schlank und selbst gehostet: Verwaltung von [Mihomo](https://github.com/MetaCubeX/mihomo) Listenern, Benutzern, Subscriptions und Laufzeitstatus unter Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Dokumentation: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Website: [https://3m-ui.top/](https://3m-ui.top/)

---

## Funktionen

| Kategorie | Fähigkeiten |
|------|------|
| **Nodes** | Protokollregister-gesteuerte Listener: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC usw.; Reality, TLS selbstsigniert, Batch-Zertifikate per Knopfdruck, **Traffic-Multiplikator**, Validierung sich gegenseitig ausschließender Transportfelder |
| **Benutzer** | **Ein-Klick-Erstellung** (zufälliger Benutzername/Passwort/UUID, optional an alle Nodes binden), Node-Bindung, Traffic-Limit, **Messung pro Node** (Abrechnung = tatsächlich × Node-Multiplikator), Ablaufdatum, IP-/Subscription-Abruf-Limits, Beginn ab erster Nutzung, zyklische Verlängerung/Zurücksetzung, Gruppen-Tags, Zusammenführen externer Subscriptions, Batch-Operationen, Subscription-Token |
| **Subscriptions** | UA-basierte automatische Erkennung von Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` immer Base64), sing-box JSON; optionales `?target=`; HTML-Subscription-Seite; TUIC/HY2-Share-Links enthalten SNI/`allow_insecure` und weitere Client-Parameter |
| **Konfiguration** | Erzeugen → Validieren → Anwenden getrennt; bei Fehler Rollback auf die vorherige `config.yaml` |
| **Telegram** | Alarme und Verwaltungsbefehle (Token + Chat ID erforderlich) |
| **Multi-Server** | Registrierung entfernter Panels, Health-Checks, Synchronisierung von Node-Spiegelungen, Zusammenführen von Subscriptions; **Push lokaler Nodes nach Namen** an entfernte Panels (standardmäßig deaktiviert, umbenennbar/Port anpassbar) |
| **Sicherheit** | JWT, Passwortänderung beim ersten Login, TOTP-Zwei-Faktor-Authentifizierung, optionales **GitHub OAuth**, web_path-Präfix, verschlüsselte Anmeldedaten, CORS |
| **Betrieb** | Start/Stop/Update des Kerns, Geo, Panel-SSL/ACME (inkl. DNS-01-Wildcard), Backup, **Cloudflare WARP** Ein-Klick-Registrierung (WireGuard/MASQUE YAML) |

### Zusätzliche Hinweise (Benutzerlimits / Zertifikate / Subscriptions)

- [Benutzerlimits: IP · Subscription-Abrufe](https://3m-ui.top/guide/users-limits)
- [Traffic pro Node und Multiplikator](https://3m-ui.top/guide/node-traffic)
- [Panel-SSL / ACME (HTTP-01, DNS-01-Wildcard)](https://3m-ui.top/guide/ssl-cert)
- [Node-Zertifikate im Batch anwenden](https://3m-ui.top/guide/batch-certificate)
- [Subscription-Formate und target](https://3m-ui.top/guide/subscription-formats)
- [Multi-Server-Nodes / Push](https://3m-ui.top/guide/cluster)
- [Routing (Client-Subscription + Server-Egress)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`), nach dem Build in eine einzige Go-Binärdatei eingebettet.

Release-Artefakte: **rein statisch gelinkt** (`CGO_ENABLED=0` + modernc SQLite), kein System-`libsqlite3` erforderlich, kompatibel mit glibc / musl.

Vollständiges Installationspaket / Docker: `linux/amd64` · `arm64`. Die erweiterte eigenständige Panel-Binärdatei unterstützt zusätzlich `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`; ein kompatibler Kernel muss selbst bereitgestellt werden.

---

## Schnellinstallation

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Die vollständige Installation unterstützt Linux amd64 / arm64 und installiert automatisch das Panel, das passende Mihomo und den Systemdienst. Erstkonfiguration und Administrator-Passwort werden vom Programm einheitlich erzeugt; bereits vorhandene Konfigurationswerte bleiben erhalten. Für andere Architekturen können die eigenständige Binärdatei und ein selbst bereitgestellter Kernel verwendet werden, siehe [detaillierte Installationsanleitung](https://3m-ui.top/guide/install).

Nach der Installation:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Aktualisierung:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Deinstallation:

```bash
sudo 3m-ui uninstall
```

### Installation per KI-Prompt

Kopieren Sie den Prompt in einen Assistenten wie ChatGPT / Claude / Cursor und nennen Sie Ihr System (z. B. Ubuntu 22.04) sowie, ob Sie bereits root-SSH-Zugang haben. Der Assistent soll ausschließlich bei der Ausführung des offiziellen Skripts helfen und es nicht in Befehle unbekannter Herkunft umschreiben. Den vollständigen Prompt und die Nutzungshinweise finden Sie in der [Dokumentation zum KI-Installations-Prompt](../ai-install-prompt.md).

Kein manuelles Kopieren langer Texte nötig: laden Sie den Klartext-Prompt mit `curl` direkt in die Zwischenablage und fügen Sie ihn ein:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Der Prompt leitet ausschließlich zur Verwendung des folgenden offiziellen Einzeilers an; alle Downloads werden gegen `SHA256SUMS` geprüft:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Den Test- bzw. Vorabversionskanal (optional, nicht Standard) finden Sie in der [Installationsdokumentation](../installation.md#ai-提示词安装).

---

## Docker-Installation

Laden Sie auf dem Linux-VPS die Datei `docker-compose.yml` aus dem Release herunter und führen Sie im selben Verzeichnis aus:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Das Image ist `ghcr.io/kazeyukiro/3m-ui:latest` und enthält das Panel sowie eine feste Mihomo-Version; `latest` folgt ausschließlich stabilen Versionen. Konfiguration, zufällige Schlüssel und das Administratorkonto werden automatisch initialisiert, das erste Passwort wird nur beim ersten Start einmalig ausgegeben. Standardmäßig wird das host-Netzwerk verwendet; geben Sie nach dem Hinzufügen von Nodes die tatsächlich genutzten TCP/UDP-Ports frei. Ebenfalls bereitgestellt werden `docker-compose.bridge.yml` sowie Hinweise zu Versions-Pinning, Zertifikatskonfiguration und persistenter Wiederherstellung, siehe [Installationsdokumentation](https://3m-ui.top/guide/install).

---

## Aus dem Quellcode bauen

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Es muss eine kompatible **Mihomo**-Binärdatei auf dem System installiert sein, deren Pfad in der Konfiguration angegeben wird.

---

## Wichtige Konfigurationspunkte

Standardpfade für Daten und Konfiguration:

| Pfad | Zweck |
|------|------|
| `/etc/3m-ui/config.yaml` | Panel-Konfiguration |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Selbstsignierte Node-Zertifikate (**bitte zusammen mit der Datenbank sichern**) |
| `/var/lib/3m-ui/mihomo/` | Mihomo-Arbeitsverzeichnis und `config.yaml` |

Verwenden Sie für JWT- und Anmeldedaten-Schlüssel eigene zufällige Werte (≥ 32 Bytes), nicht die Beispiele aus der Dokumentation.

Eigenen Panel-Port festlegen: `3m-ui config port <port>` oder die Konfiguration bearbeiten und den Dienst neu starten.

Für neue Nodes wird standardmäßig ein zufälliger Listener-Port zwischen 10000 und 60000 eingetragen (leichtgewichtige Stichprobe, die Ports bereits geladener Nodes meidet; absteigende Bereiche werden unterstützt); der Port kann manuell geändert oder über „随机端口" neu gesetzt werden.

---

## Architekturdiagramm

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

## Backup-Empfehlungen

Sichern Sie mindestens:

1. `3m-ui.db`
2. `listener-certs/`
3. Panel-`config.yaml` und das Mihomo-Konfigurationsverzeichnis

Bei einem reinen Binär-Update werden Zertifikate nach Möglichkeit von der Festplatte hydrate; fehlt das Verzeichnis, werden sie neu selbstsigniert und Clients müssen die Subscription erneut abrufen.

---

## Danksagung

Besonderer Dank gilt den folgenden Open-Source-Projekten und Communities:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — Proxy-Kern-Engine und Listener-Konfigurationsmodell
- [acme.sh](https://github.com/acmesh-official/acme.sh) — Beantragung, Bereitstellung und Verlängerung von Zertifikaten im SSH-Menü
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener-Konfigurationsbeispiele und Protokollreferenz
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — Referenz für Panel-Interaktion und Betriebskonzepte
- [Gin](https://github.com/gin-gonic/gin) — HTTP-Framework für das Backend
- [GORM](https://github.com/go-gorm/gorm) — Datenbank-ORM
- [React](https://github.com/facebook/react) — Frontend-Grundlage
- [Ant Design](https://github.com/ant-design/ant-design) — UI-Komponenten
- [Lucide](https://github.com/lucide-icons/lucide) — Icons (`lucide-react`, angepasst über `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Referenz für Mihomo-Routing-Konfiguration und schlanke Regelansätze
- [echs-top/proxy](https://github.com/echs-top/proxy) — Referenz für Mihomo-Regeln und Proxy-Gruppen-Schemata
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — Referenz für Regelsets und mihomo-Routing-Vorlagen
- [Zustand](https://github.com/pmndrs/zustand) — State-Management im Frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js sowie die gesamte Open-Source-Community

Mitwirkende: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Lizenz

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Dieses Projekt steht ab Commit `226c460` unter der **GNU General Public License v3.0**. Frühere Commits und bereits veröffentlichte Versionen unterliegen weiterhin der **Eclipse Public License 2.0 (EPL-2.0)**; diese Lizenzierung kann nicht widerrufen werden.

[GNU General Public License v3.0](../../LICENSE)

Drittkomponenten: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## English & other languages

Vollständige Übersetzungen (18 Sprachen): siehe die Sprachlinks oben oder öffnen Sie [README.en.md](./README.en.md).
