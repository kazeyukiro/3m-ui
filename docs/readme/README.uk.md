# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · **Українська**

**Веб-панель керування сервером Mihomo**

> **Пріоритет стабільної версії:** встановлення та оновлення за замовчуванням використовують лише офіційні Release. Передрелізні версії потрібно вибирати явно. Повні інсталяційні пакети та Docker-образи містять зафіксовану версію Mihomo; докладніше див. [Встановлення, оновлення та відновлення](https://3m-ui.top/guide/install).

Легка, self-hosted панель для керування Listener, користувачами, підписками та станом роботи [Mihomo](https://github.com/MetaCubeX/mihomo) на Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Документація: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Сайт: [https://3m-ui.top/](https://3m-ui.top/) 

---

## Можливості

| Категорія | Можливості |
|------|------|
| **Вузли** | Listener на основі реєстру протоколів: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC та ін.; Reality, самопідписаний TLS, масове застосування сертифікатів одним кліком, **множник трафіку**, перевірка взаємовиключних полів транспортного рівня |
| **Користувачі** | **Створення одним кліком** (випадкові ім'я користувача / пароль / UUID, необов'язкова прив'язка до всіх вузлів), прив'язка вузлів, ліміт трафіку, **окремий облік по вузлах** (тарифікація = фактичний трафік × множник вузла), термін дії, обмеження за IP та кількістю звернень до підписки, відлік від першого використання, періодичне продовження/скидання, групові мітки, об'єднання зовнішніх підписок, масові операції, Token підписки |
| **Підписки** | Автовизначення за UA: Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` завжди Base64), sing-box JSON; необов'язковий `?target=`; HTML-сторінка підписки; посилання для TUIC/HY2 містять SNI/`allow_insecure` та інші параметри клієнта |
| **Конфігурація** | Розділено етапи: генерація → перевірка → застосування; у разі збою — відкат до попереднього `config.yaml` |
| **Telegram** | Сповіщення та команди керування (потрібні Token + Chat ID) |
| **Мультисервер** | Реєстрація віддалених панелей, перевірка стану, синхронізація дзеркал вузлів, об'єднання підписок; **вибір локальних вузлів за назвою та відправка** на віддалений сервер (за замовчуванням вимкнено, можна перейменувати та змінити порт) |
| **Безпека** | JWT, обов'язкова зміна пароля під час першого входу, двофакторна автентифікація TOTP, опціональний **GitHub OAuth**, префікс шляху web_path, шифрування облікових даних, CORS |
| **Експлуатація** | Запуск/зупинка та оновлення ядра, Geo, SSL/ACME панелі (зокрема DNS-01 wildcard), резервне копіювання, **Cloudflare WARP** — реєстрація одним кліком (WireGuard/MASQUE YAML) |

### Додаткові відомості (обмеження користувачів / сертифікати / підписки)

- [Обмеження користувачів: IP · звернення до підписки](https://3m-ui.top/guide/users-limits)
- [Трафік по вузлах і множник](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME панелі (HTTP-01, DNS-01 wildcard)](https://3m-ui.top/guide/ssl-cert)
- [Масове застосування сертифікатів вузлів](https://3m-ui.top/guide/batch-certificate)
- [Формати підписок і target](https://3m-ui.top/guide/subscription-formats)
- [Вузли на кількох серверах / відправка](https://3m-ui.top/guide/cluster)
- [Маршрутизація (клієнтські підписки + серверний вихідний трафік)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Фронтенд: **React + Ant Design + Lucide Icons** (`frontend/`), після збірки вбудовується в єдиний бінарний файл Go.

Артефакти релізу: **повністю статичне компонування** (`CGO_ENABLED=0` + modernc SQLite), системний `libsqlite3` не потрібен, сумісно з glibc / musl.

Повний інсталяційний пакет / Docker: `linux/amd64` · `arm64`. Розширений автономний бінарник панелі додатково підтримує `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` — потрібно самостійно підготувати сумісне ядро.

---

## Швидке встановлення

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Повне встановлення підтримує Linux amd64 / arm64, автоматично встановлює панель, сумісний Mihomo та системну службу. Первинна конфігурація і пароль адміністратора генеруються програмою єдиним чином; якщо конфігурація вже існує, попередні значення зберігаються. Для інших архітектур можна використати автономний бінарник і самостійно підготовлене ядро — див. [докладні інструкції зі встановлення](https://3m-ui.top/guide/install).

Після завершення встановлення:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Оновлення:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Видалення:

```bash
sudo 3m-ui uninstall
```

### Встановлення за допомогою AI-підказки

Скопіюйте підказку в ChatGPT / Claude / Cursor або інший помічник і вкажіть вашу систему (наприклад, Ubuntu 22.04), а також чи є у вас root-доступ по SSH. Помічник має лише допомагати виконати офіційний скрипт і не переписувати його на команди з невідомих джерел. Повний текст підказки та інструкції щодо використання див. у [документації з AI-підказки встановлення](../ai-install-prompt.md).

Не потрібно копіювати великий фрагмент тексту вручну — завантажте текстову підказку прямо в буфер обміну за допомогою `curl`, а потім вставте її:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Підказка спрямовує лише на використання наведеного нижче офіційного скрипта встановлення; усі завантаження перевіряються за `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Тестовий канал / канал передрелізних версій (необов'язково, не за замовчуванням) — див. [документацію зі встановлення](../installation.md#ai-提示词安装).

---

## Встановлення Docker

На Linux VPS завантажте `docker-compose.yml` з випуску і в тому самому каталозі виконайте:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Образ — `ghcr.io/kazeyukiro/3m-ui:latest`, він містить панель і зафіксовану версію Mihomo; `latest` стежить лише за стабільними версіями. Конфігурація, випадкові ключі та обліковий запис адміністратора ініціалізуються автоматично, пароль першого запуску виводиться лише один раз. За замовчуванням використовується мережа host — після додавання вузлів відкрийте фактично використовувані TCP/UDP-порти. Також надається `docker-compose.bridge.yml`, а також інструкції щодо фіксації версії, налаштування сертифікатів і відновлення з постійного сховища — див. [документацію зі встановлення](https://3m-ui.top/guide/install).

---

## Збірка з джерел

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Потрібен уже встановлений на цій машині сумісний бінарний файл **Mihomo**, і в конфігурації має бути вказано шлях до нього.

---

## Основні відомості про конфігурацію

Дані та конфігурація за замовчуванням розташовуються приблизно тут:

| Шлях | Призначення |
|------|------|
| `/etc/3m-ui/config.yaml` | Конфігурація панелі |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Самопідписані сертифікати вузлів (**обов'язково копіюйте разом із базою даних**) |
| `/var/lib/3m-ui/mihomo/` | Робочий каталог Mihomo та `config.yaml` |

Для JWT і ключів облікових даних використовуйте незалежні випадкові значення (≥ 32 байти), не використовуйте приклади з документації.

Нетиповий порт панелі: `3m-ui config port <port>` або відредагуйте конфігурацію й перезапустіть службу.
Для нових вузлів за замовчуванням випадковим чином заповнюється порт прослуховування в діапазоні 10000～60000 (легка вибірка з обходом портів уже завантажених вузлів; підтримується зворотний порядок діапазону) — можна змінити вручну або натиснути «随机端口».

---

## Схема архітектури

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

## Рекомендації щодо резервного копіювання

Як мінімум створюйте резервні копії:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` панелі та каталогу конфігурації Mihomo

Під час оновлення лише бінарного файла сертифікати за можливості відновлюються (hydrate) з диска; якщо каталог втрачено, їх буде перепідписано заново, і клієнтам потрібно буде повторно отримати підписку.

---

## Подяки

Особлива подяка таким проєктам з відкритим кодом і спільнотам:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — ядро проксі-рушія та модель конфігурації Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — випуск, розгортання та продовження сертифікатів у SSH-меню
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — приклади конфігурації Listener і довідник протоколів
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — ідеї щодо взаємодії панелі та прийомів експлуатації
- [Gin](https://github.com/gin-gonic/gin) — HTTP-фреймворк бекенду
- [GORM](https://github.com/go-gorm/gorm) — ORM для бази даних
- [React](https://github.com/facebook/react) — основа фронтенду
- [Ant Design](https://github.com/ant-design/ant-design) — UI-компоненти
- [Lucide](https://github.com/lucide-icons/lucide) — іконки (`lucide-react`, адаптовано через `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — ідеї щодо конфігурації маршрутизації Mihomo та спрощених правил
- [echs-top/proxy](https://github.com/echs-top/proxy) — ідеї щодо правил Mihomo та схем груп політик
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — ідеї щодо наборів правил і шаблонів маршрутизації mihomo
- [Zustand](https://github.com/pmndrs/zustand) — керування станом фронтенду
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js та вся спільнота відкритого ПЗ

Учасники: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Ліцензія

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Починаючи з commit `226c460` цей проєкт розповсюджується за ліцензією **GNU General Public License v3.0**. Попередні коміти та вже опубліковані випуски залишаються під дією **Eclipse Public License 2.0 (EPL-2.0)**, і ця ліцензія не може бути відкликана.

[GNU General Public License v3.0](../../LICENSE)

Відомості про компоненти третіх сторін див. у [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## Іншими мовами

Повні переклади (18 мов): див. посилання на мови на початку документа або відкрийте [README.en.md](./README.en.md).
