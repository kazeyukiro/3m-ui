# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · **Русский** · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Веб-панель управления сервером Mihomo**

> **Приоритет стабильной версии:** установка и обновление по умолчанию используют только официальные Release. Предварительные релизы нужно выбирать явно. Полные установочные пакеты и Docker-образы содержат фиксированную версию Mihomo; подробнее см. [Установка, обновление и восстановление](https://3m-ui.top/guide/install).

Лёгкая, самохостируемая панель для управления Listener, пользователями, подписками и состоянием работы [Mihomo](https://github.com/MetaCubeX/mihomo) на Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Документация: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Сайт: [https://3m-ui.top/](https://3m-ui.top/) 

---

## Возможности

| Категория | Возможности |
|------|------|
| **Узлы** | Listener на основе реестра протоколов: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC и др.; Reality, самоподписанный TLS, массовое применение сертификатов одним кликом, **множитель трафика**, проверка взаимоисключающих полей транспортного уровня |
| **Пользователи** | **Создание в один клик** (случайные имя пользователя / пароль / UUID, необязательная привязка ко всем узлам), привязка узлов, лимит трафика, **раздельный учёт по узлам** (тарификация = фактический трафик × множитель узла), срок действия, ограничения по IP и числу обращений к подписке, отсчёт от первого использования, периодическое продление/сброс, групповые метки, объединение внешних подписок, массовые операции, Token подписки |
| **Подписки** | Автоопределение по UA: Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` всегда Base64), sing-box JSON; необязательный `?target=`; HTML-страница подписки; ссылки для TUIC/HY2 содержат SNI/`allow_insecure` и другие параметры клиента |
| **Конфигурация** | Разделены этапы: генерация → проверка → применение; при сбое — откат к предыдущему `config.yaml` |
| **Telegram** | Оповещения и команды управления (нужны Token + Chat ID) |
| **Несколько серверов** | Регистрация удалённых панелей, проверка состояния, синхронизация зеркал узлов, объединение подписок; **выбор локальных узлов по имени и отправка** на удалённый сервер (по умолчанию отключено, можно переименовать и изменить порт) |
| **Безопасность** | JWT, обязательная смена пароля при первом входе, двухфакторная аутентификация TOTP, опциональный **GitHub OAuth**, префикс пути web_path, шифрование учётных данных, CORS |
| **Эксплуатация** | Запуск/остановка и обновление ядра, Geo, SSL/ACME панели (включая DNS-01 wildcard), резервное копирование, **Cloudflare WARP** — регистрация в один клик (WireGuard/MASQUE YAML) |

### Дополнительные сведения (ограничения пользователей / сертификаты / подписки)

- [Ограничения пользователей: IP · обращения к подписке](https://3m-ui.top/guide/users-limits)
- [Трафик по узлам и множитель](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME панели (HTTP-01, DNS-01 wildcard)](https://3m-ui.top/guide/ssl-cert)
- [Массовое применение сертификатов узлов](https://3m-ui.top/guide/batch-certificate)
- [Форматы подписок и target](https://3m-ui.top/guide/subscription-formats)
- [Узлы на нескольких серверах / отправка](https://3m-ui.top/guide/cluster)
- [Маршрутизация (клиентские подписки + серверный исходящий трафик)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Фронтенд: **React + Ant Design + Lucide Icons** (`frontend/`), после сборки встраивается в единственный бинарный файл Go.

Артефакты релиза: **полностью статическая компоновка** (`CGO_ENABLED=0` + modernc SQLite), системный `libsqlite3` не требуется, совместимо с glibc / musl.

Полный установочный пакет / Docker: `linux/amd64` · `arm64`. Расширенный автономный бинарник панели дополнительно поддерживает `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` — требуется самостоятельно подготовить совместимое ядро.

---

## Быстрая установка

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Полная установка поддерживает Linux amd64 / arm64, автоматически устанавливает панель, совместимый Mihomo и системную службу. Первичная конфигурация и пароль администратора генерируются программой единообразно; если конфигурация уже существует, прежние значения сохраняются. Для других архитектур можно использовать автономный бинарник и самостоятельно подготовленное ядро — см. [подробные инструкции по установке](https://3m-ui.top/guide/install).

После завершения установки:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Обновление:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Удаление:

```bash
sudo 3m-ui uninstall
```

### Установка с помощью AI-подсказки

Скопируйте подсказку в ChatGPT / Claude / Cursor или другой助手 и укажите вашу систему (например, Ubuntu 22.04), а также есть ли у вас root-доступ по SSH. Помощник должен лишь помогать выполнить официальный скрипт и не переписывать его в команды из неизвестных источников. Полный текст подсказки и инструкции по применению см. в [документации по AI-подсказке установки](../ai-install-prompt.md).

Не нужно копировать большой фрагмент текста вручную — загрузите текстовую подсказку прямо в буфер обмена с помощью `curl`, а затем вставьте её:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Подсказка рекомендует использовать только приведённый ниже официальный скрипт установки; все загрузки проверяются по `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Тестовый канал / канал предварительных релизов (необязательно, не по умолчанию) — см. [документацию по установке](../installation.md#ai-提示词安装).

---

## Установка Docker

На Linux VPS скачайте `docker-compose.yml` из выпуска и в том же каталоге выполните:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Образ — `ghcr.io/kazeyukiro/3m-ui:latest`, он содержит панель и фиксированную версию Mihomo; `latest` следует только за стабильными версиями. Конфигурация, случайные ключи и учётная запись администратора инициализируются автоматически, пароль первого запуска выводится только один раз. По умолчанию используется сеть host — после добавления узлов откройте фактически используемые TCP/UDP-порты. Также предоставляется `docker-compose.bridge.yml`, а также инструкции по фиксации версии, настройке сертификатов и восстановлению из постоянного хранилища — см. [документацию по установке](https://3m-ui.top/guide/install).

---

## Сборка из исходников

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Требуется уже установленный на этой машине совместимый бинарный файл **Mihomo**, и в конфигурации должен быть указан путь к нему.

---

## Основные сведения о конфигурации

Данные и конфигурация по умолчанию располагаются примерно здесь:

| Путь | Назначение |
|------|------|
| `/etc/3m-ui/config.yaml` | Конфигурация панели |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Самоподписанные сертификаты узлов (**обязательно копируйте вместе с базой данных**) |
| `/var/lib/3m-ui/mihomo/` | Рабочий каталог Mihomo и `config.yaml` |

Для JWT и ключей учётных данных используйте независимые случайные значения (≥ 32 байт), не используйте примеры из документации.

Пользовательский порт панели: `3m-ui config port <port>` либо отредактируйте конфигурацию и перезапустите службу.
Для новых узлов по умолчанию случайным образом заполняется порт прослушивания в диапазоне 10000～60000 (лёгкая выборка с обходом портов уже загруженных узлов; поддерживается обратный порядок диапазона) — можно изменить вручную или нажать «随机端口».

---

## Схема архитектуры

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

## Рекомендации по резервному копированию

Как минимум создавайте резервные копии:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` панели и каталога конфигурации Mihomo

При обновлении только бинарного файла сертификаты по возможности восстанавливаются (hydrate) с диска; если каталог утерян, они будут переподписаны заново, и клиентам потребуется повторно получить подписку.

---

## Благодарности

Особая благодарность следующим проектам с открытым исходным кодом и сообществам:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — ядро прокси-движка и модель конфигурации Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — выпуск, развёртывание и продление сертификатов в SSH-меню
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — примеры конфигурации Listener и справочник по протоколам
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — идеи по взаимодействию панели и приёмам эксплуатации
- [Gin](https://github.com/gin-gonic/gin) — HTTP-фреймворк бэкенда
- [GORM](https://github.com/go-gorm/gorm) — ORM для базы данных
- [React](https://github.com/facebook/react) — основа фронтенда
- [Ant Design](https://github.com/ant-design/ant-design) — UI-компоненты
- [Lucide](https://github.com/lucide-icons/lucide) — иконки (`lucide-react`, адаптированы через `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — идеи по конфигурации маршрутизации Mihomo и упрощённым правилам
- [echs-top/proxy](https://github.com/echs-top/proxy) — идеи по правилам Mihomo и схемам групп политик
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — идеи по наборам правил и шаблонам маршрутизации mihomo
- [Zustand](https://github.com/pmndrs/zustand) — управление состоянием фронтенда
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js и всё сообщество открытого ПО

Участники: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Лицензия

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Начиная с commit `226c460` этот проект распространяется по лицензии **GNU General Public License v3.0**. Предыдущие коммиты и уже опубликованные выпуски остаются под действием **Eclipse Public License 2.0 (EPL-2.0)**, и эта лицензия не может быть отозвана.

[GNU General Public License v3.0](../../LICENSE)

Сведения о компонентах третьих сторон см. в [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## На других языках

Полные переводы (18 языков): см. ссылки на языки в начале документа или откройте [README.en.md](./README.en.md).
