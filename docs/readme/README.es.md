# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Idiomas / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · **Español** · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Panel web de administración para el servidor Mihomo**

> **Primero la versión estable:** la instalación y la actualización usan por defecto únicamente releases oficiales. Las pre-releases deben elegirse explícitamente. Los paquetes de instalación completos y las imágenes Docker incluyen una versión fija de Mihomo; consulte [Instalación, actualización y restauración](https://3m-ui.top/guide/install).

Ligero y autoalojado, para gestionar Listener, usuarios, suscripciones y el estado de ejecución de [Mihomo](https://github.com/MetaCubeX/mihomo) en Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Documentación: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Sitio web: [https://3m-ui.top/](https://3m-ui.top/)

---

## Funciones

| Categoría | Capacidades |
|------|------|
| **Nodos** | Listener basados en un registro de protocolos: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, etc.; Reality, TLS autofirmado, certificados por lotes en un clic, **multiplicador de tráfico**, validación de campos de transporte mutuamente excluyentes |
| **Usuarios** | **Creación en un clic** (nombre de usuario/contraseña/UUID aleatorios, opcionalmente vinculando todos los nodos), vinculación de nodos, límite de tráfico, **medición por nodo** (facturación = real × multiplicador del nodo), vencimiento, límites de IP / descarga de suscripción, inicio desde el primer uso, renovación/restablecimiento por ciclo, etiquetas de grupo, fusión de suscripciones externas, operaciones por lotes, Token de suscripción |
| **Suscripciones** | Detección automática por UA de Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` siempre Base64), sing-box JSON; `?target=` opcional; página de suscripción HTML; los enlaces de uso compartido de TUIC/HY2 incluyen SNI/`allow_insecure` y otros parámetros de cliente |
| **Configuración** | Generar → validar → aplicar, pasos separados; reversión a la anterior `config.yaml` en caso de fallo |
| **Telegram** | Alertas y comandos de administración (requiere Token + Chat ID) |
| **Multi-servidor** | Registro de paneles remotos, comprobación de estado, sincronización de nodos espejo, fusión de suscripciones; **push de nodos locales por nombre** hacia los remotos (desactivado por defecto, se puede renombrar/cambiar puerto) |
| **Seguridad** | JWT, cambio de contraseña en el primer inicio de sesión, verificación en dos pasos TOTP, **GitHub OAuth** opcional, prefijo de ruta web_path, cifrado de credenciales, CORS |
| **Operación** | Inicio/parada/actualización del núcleo, Geo, SSL/ACME del panel (incluidos comodines DNS-01), copia de seguridad, registro de **Cloudflare WARP** en un clic (WireGuard/MASQUE YAML) |

### Notas adicionales (límites de usuario / certificados / suscripciones)

- [Límites de usuario: IP · descargas de suscripción](https://3m-ui.top/guide/users-limits)
- [Tráfico por nodo y multiplicador](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME del panel (HTTP-01, comodín DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [Aplicar certificados de nodo por lotes](https://3m-ui.top/guide/batch-certificate)
- [Formatos de suscripción y target](https://3m-ui.top/guide/subscription-formats)
- [Nodos multi-servidor / push](https://3m-ui.top/guide/cluster)
- [Enrutamiento (suscripción de cliente + salida de servidor)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`), que tras la compilación se incrusta en un único binario de Go.

Artefactos de release: **vinculación totalmente estática** (`CGO_ENABLED=0` + modernc SQLite), sin necesidad de `libsqlite3` del sistema, compatible con glibc / musl.

Paquete de instalación completo / Docker: `linux/amd64` · `arm64`. El binario de panel independiente avanzado admite además `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`; debe preparar por su cuenta un kernel compatible.

---

## Instalación rápida

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

La instalación completa admite Linux amd64 / arm64 e instala automáticamente el panel, el Mihomo correspondiente y el servicio del sistema. La configuración inicial y la contraseña de administrador las genera el programa de forma uniforme; si la configuración ya existe, se conservan los valores. Para otras arquitecturas se pueden usar el binario independiente y un kernel preparado por su cuenta, consulte [instrucciones detalladas de instalación](https://3m-ui.top/guide/install).

Tras la instalación:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Actualización:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Desinstalación:

```bash
sudo 3m-ui uninstall
```

### Instalación con prompt de IA

Copie el prompt en un asistente como ChatGPT / Claude / Cursor e indique su sistema (p. ej. Ubuntu 22.04) y si ya dispone de SSH root. El asistente solo debe ayudar a ejecutar el script oficial y no reescribirlo en comandos de origen desconocido. El prompt completo y las notas de uso están en la [documentación del prompt de instalación con IA](../ai-install-prompt.md).

No hace falta copiar manualmente un texto largo: cargue el prompt en texto plano directamente al portapapeles con `curl` y péguelo:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

El prompt solo guía hacia el script oficial de una línea siguiente; todas las descargas se verifican con `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

El canal de prueba / pre-release (opcional, no predeterminado) está en la [documentación de instalación](../installation.md#ai-提示词安装).

---

## Instalación con Docker

En un VPS Linux, descargue el archivo `docker-compose.yml` de la release y ejecute en el mismo directorio:

```bash
docker compose up -d
docker compose logs 3m-ui
```

La imagen es `ghcr.io/kazeyukiro/3m-ui:latest` e incluye el panel y una versión fija de Mihomo; `latest` solo sigue las versiones estables. La configuración, las claves aleatorias y el administrador se inicializan automáticamente; la contraseña inicial se muestra una sola vez en el primer arranque. Por defecto se usa la red host; tras añadir nodos, abra los puertos TCP/UDP realmente utilizados. También se ofrece `docker-compose.bridge.yml`, junto con instrucciones sobre fijación de versión, configuración de certificados y restauración persistente, consulte [documentación de instalación](https://3m-ui.top/guide/install).

---

## Compilar desde el código fuente

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Debe tener instalado localmente un binario **Mihomo** compatible y apuntar a su ruta en la configuración.

---

## Puntos de configuración

Los datos y la configuración por defecto se encuentran aproximadamente en:

| Ruta | Uso |
|------|------|
| `/etc/3m-ui/config.yaml` | Configuración del panel |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Certificados autofirmados de nodos (**haga copia de seguridad junto con la base de datos**) |
| `/var/lib/3m-ui/mihomo/` | Directorio de trabajo de Mihomo y `config.yaml` |

Use valores aleatorios independientes (≥ 32 bytes) para las claves de JWT y credenciales; no use los ejemplos de la documentación.

Puerto de panel personalizado: `3m-ui config port <port>` o edite la configuración y reinicie el servicio.

Los nodos nuevos rellenan por defecto un puerto de escucha aleatorio entre 10000 y 60000 (muestreo ligero que evita los puertos de nodos ya cargados; se admiten rangos descendentes); se puede modificar manualmente o pulsar «随机端口».

---

## Diagrama de arquitectura

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

## Recomendaciones de copia de seguridad

Haga copia de seguridad al menos de:

1. `3m-ui.db`
2. `listener-certs/`
3. El `config.yaml` del panel y el directorio de configuración de Mihomo

Al actualizar solo el binario, los certificados se hydrate desde el disco en la medida de lo posible; si el directorio se pierde, se vuelven a autofirmar y los clientes deben volver a descargar la suscripción.

---

## Agradecimientos

Agradecimiento especial a los siguientes proyectos de código abierto y comunidades:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — motor proxy principal y modelo de configuración de Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — solicitud, despliegue y renovación de certificados en el menú SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — ejemplos de configuración de Listener y referencia de protocolos
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — referencia de interacción del panel e ideas de operación
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP del backend
- [GORM](https://github.com/go-gorm/gorm) — ORM de base de datos
- [React](https://github.com/facebook/react) — base del frontend
- [Ant Design](https://github.com/ant-design/ant-design) — componentes de UI
- [Lucide](https://github.com/lucide-icons/lucide) — iconos (`lucide-react`, adaptado mediante `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — referencia de configuración de enrutamiento de Mihomo e ideas de reglas ligeras
- [echs-top/proxy](https://github.com/echs-top/proxy) — referencia de reglas de Mihomo y esquemas de grupos de proxies
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — referencia de conjuntos de reglas y plantillas de enrutamiento mihomo
- [Zustand](https://github.com/pmndrs/zustand) — gestión de estado del frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js y toda la comunidad de código abierto

Colaboradores: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Licencia

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

A partir del commit `226c460`, este proyecto adopta la **GNU General Public License v3.0**. Los commits anteriores y las versiones ya publicadas siguen sujetos a la **Eclipse Public License 2.0 (EPL-2.0)**, licencia que no puede revocarse.

[GNU General Public License v3.0](../../LICENSE)

Componentes de terceros: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## English & other languages

Traducciones completas (18 idiomas): consulte los enlaces de idioma de la parte superior o abra [README.en.md](./README.en.md).
