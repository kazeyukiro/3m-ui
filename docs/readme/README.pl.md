# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · **Polski** · [Українська](./README.uk.md)

**Panel WWW do zarządzania serwerem Mihomo**

> **Priorytet wersji stabilnej:** instalacja i aktualizacja domyślnie korzystają wyłącznie z oficjalnych Release. Wersje przedpremierowe trzeba wybrać jawnie. Pełne pakiety instalacyjne i obrazy Docker zawierają ustaloną wersję Mihomo; szczegóły w dokumencie [Instalacja, aktualizacja i przywracanie](https://3m-ui.top/guide/install).

Lekki, self-hosted panel do zarządzania Listener, użytkownikami, subskrypcjami i stanem działania [Mihomo](https://github.com/MetaCubeX/mihomo) na Linuksie.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Dokumentacja: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Strona: [https://3m-ui.top/](https://3m-ui.top/) 

---

## Funkcje

| Kategoria | Możliwości |
|------|------|
| **Węzły** | Listener oparty na rejestrze protokołów: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC itd.; Reality, samopodpisany TLS, masowe zastosowanie certyfikatów jednym kliknięciem, **mnożnik ruchu**, walidacja wzajemnie wykluczających się pól warstwy transportowej |
| **Użytkownicy** | **Tworzenie jednym kliknięciem** (losowa nazwa użytkownika / hasło / UUID, opcjonalne przypisanie do wszystkich węzłów), przypisywanie węzłów, limit ruchu, **oddzielne liczenie per węzeł** (naliczanie = rzeczywiste × mnożnik węzła), data wygaśnięcia, limity IP / pobrań subskrypcji, liczenie od pierwszego użycia, cykliczne odnowienie/reset, etykiety grup, scalanie zewnętrznych subskrypcji, operacje masowe, Token subskrypcji |
| **Subskrypcje** | Automatyczne rozpoznanie po UA: Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` zawsze Base64), sing-box JSON; opcjonalny `?target=`; strona HTML subskrypcji; linki udostępniania TUIC/HY2 zawierają SNI/`allow_insecure` i inne parametry klienta |
| **Konfiguracja** | Rozdzielone etapy: generowanie → walidacja → zastosowanie; w razie błędu — powrót do poprzedniego `config.yaml` |
| **Telegram** | Alerty i polecenia zarządzania (wymagane Token + Chat ID) |
| **Wiele serwerów** | Rejestracja zdalnych paneli, sprawdzanie stanu, synchronizacja lustrzanych węzłów, łączenie subskrypcji; **wybór lokalnych węzłów po nazwie i wysyłka** na zdalny serwer (domyślnie wyłączone, można zmienić nazwę i port) |
| **Bezpieczeństwo** | JWT, obowiązkowa zmiana hasła przy pierwszym logowaniu, uwierzytelnianie dwuskładnikowe TOTP, opcjonalny **GitHub OAuth**, prefiks ścieżki web_path, szyfrowanie poświadczeń, CORS |
| **Utrzymanie** | Uruchamianie/zatrzymywanie i aktualizacja rdzenia, Geo, SSL/ACME panelu (w tym DNS-01 wildcard), kopie zapasowe, **Cloudflare WARP** — rejestracja jednym kliknięciem (WireGuard/MASQUE YAML) |

### Informacje dodatkowe (limity użytkowników / certyfikaty / subskrypcje)

- [Limity użytkowników: IP · pobrania subskrypcji](https://3m-ui.top/guide/users-limits)
- [Ruch per węzeł i mnożnik](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME panelu (HTTP-01, DNS-01 wildcard)](https://3m-ui.top/guide/ssl-cert)
- [Masowe zastosowanie certyfikatów węzłów](https://3m-ui.top/guide/batch-certificate)
- [Formaty subskrypcji i target](https://3m-ui.top/guide/subscription-formats)
- [Węzły na wielu serwerach / wysyłka](https://3m-ui.top/guide/cluster)
- [Routing (subskrypcje klienckie + ruch wychodzący serwera)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`), po zbudowaniu osadzany w jednym pliku binarnym Go.

Artefakty wydania: **w pełni statyczne linkowanie** (`CGO_ENABLED=0` + modernc SQLite), systemowy `libsqlite3` nie jest wymagany, zgodne z glibc / musl.

Pełny pakiet instalacyjny / Docker: `linux/amd64` · `arm64`. Rozszerzony samodzielny plik binarny panelu obsługuje dodatkowo `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` — trzeba samodzielnie przygotować kompatybilne jądro.

---

## Szybka instalacja

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Pełna instalacja obsługuje Linux amd64 / arm64, automatycznie instaluje panel, pasujący Mihomo i usługę systemową. Pierwotna konfiguracja i hasło administratora są generowane jednolicie przez program; jeśli konfiguracja już istnieje, poprzednie wartości zostają zachowane. Dla innych architektur można użyć samodzielnego pliku binarnego i własnego jądra — zob. [szczegółowe instrukcje instalacji](https://3m-ui.top/guide/install).

Po zakończeniu instalacji:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Aktualizacja:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Odinstalowanie:

```bash
sudo 3m-ui uninstall
```

### Instalacja z promptem AI

Skopiuj prompt do ChatGPT / Claude / Cursor lub innego asystenta i podaj swój system (np. Ubuntu 22.04) oraz to, czy masz dostęp SSH na roota. Asystent powinien jedynie pomóc wykonać oficjalny skrypt, a nie przepisywać go na polecenia z nieznanych źródeł. Pełny prompt i instrukcje użycia znajdziesz w [dokumentacji promptu instalacji AI](../ai-install-prompt.md).

Nie trzeba ręcznie kopiować długiego tekstu — wczytaj prompt w postaci czystego tekstu bezpośrednio do schowka za pomocą `curl`, a potem go wklej:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Prompt prowadzi wyłącznie do użycia poniższego oficjalnego skryptu instalacji; wszystkie pobrania są weryfikowane przez `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Kanał testowy / przedpremierowy (opcjonalny, nie domyślny) — zob. [dokumentację instalacji](../installation.md#ai-提示词安装).

---

## Instalacja Docker

Na Linux VPS pobierz `docker-compose.yml` z wydania i w tym samym katalogu wykonaj:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Obraz to `ghcr.io/kazeyukiro/3m-ui:latest`, zawiera panel i ustaloną wersję Mihomo; `latest` podąża tylko za wersjami stabilnymi. Konfiguracja, losowe klucze i konto administratora są inicjowane automatycznie, a hasło pierwszego uruchomienia jest wypisywane tylko raz. Domyślnie używana jest sieć host — po dodaniu węzłów otwórz faktycznie używane porty TCP/UDP. Dostępny jest też `docker-compose.bridge.yml` oraz instrukcje dotyczące pinowania wersji, konfiguracji certyfikatów i przywracania z trwałego wolumenu — zob. [dokumentację instalacji](https://3m-ui.top/guide/install).

---

## Kompilacja ze źródeł

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Wymagany jest już zainstalowany na tej maszynie kompatybilny plik binarny **Mihomo**, a w konfiguracji musi być wskazana ścieżka do niego.

---

## Najważniejsze informacje o konfiguracji

Dane i pliki konfiguracyjne znajdują się domyślnie mniej więcej tutaj:

| Ścieżka | Przeznaczenie |
|------|------|
| `/etc/3m-ui/config.yaml` | Konfiguracja panelu |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Samopodpisane certyfikaty węzłów (**koniecznie twórz kopię razem z bazą danych**) |
| `/var/lib/3m-ui/mihomo/` | Katalog roboczy Mihomo i `config.yaml` |

Dla JWT i kluczy poświadczeń używaj niezależnych wartości losowych (≥ 32 bajty), nie korzystaj z przykładów z dokumentacji.

Własny port panelu: `3m-ui config port <port>` albo edytuj konfigurację i zrestartuj usługę.
Dla nowych węzłów port nasłuchu jest domyślnie losowany z zakresu 10000～60000 (lekkie próbkowanie z pominięciem portów już załadowanych węzłów; obsługiwany jest zakres w odwrotnej kolejności) — można go zmienić ręcznie lub kliknąć «随机端口».

---

## Schemat architektury

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

## Zalecenia dotyczące kopii zapasowych

Twórz kopie zapasowe co najmniej:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` panelu i katalogu konfiguracji Mihomo

Przy aktualizacji samego pliku binarnego certyfikaty są w miarę możliwości odtwarzane (hydrate) z dysku; jeśli katalog zostanie utracony, zostaną podpisane ponownie, a klienci będą musieli pobrać subskrypcję na nowo.

---

## Podziękowania

Szczególne podziękowania dla następujących projektów open source i społeczności:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — silnik proxy i model konfiguracji Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — wystawianie, wdrażanie i odnawianie certyfikatów w menu SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — przykłady konfiguracji Listener i referencja protokołów
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — inspiracje dotyczące interakcji panelu i sposobów utrzymania
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP backendu
- [GORM](https://github.com/go-gorm/gorm) — ORM do bazy danych
- [React](https://github.com/facebook/react) — podstawa frontendu
- [Ant Design](https://github.com/ant-design/ant-design) — komponenty UI
- [Lucide](https://github.com/lucide-icons/lucide) — ikony (`lucide-react`, zaadaptowane przez `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — inspiracje dotyczące konfiguracji routingu Mihomo i uproszczonych reguł
- [echs-top/proxy](https://github.com/echs-top/proxy) — inspiracje dotyczące reguł Mihomo i schematów grup proxy
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — inspiracje dotyczące zestawów reguł i szablonów routingu mihomo
- [Zustand](https://github.com/pmndrs/zustand) — zarządzanie stanem frontendu
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js i cała społeczność open source

Współtwórcy: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Licencja

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Począwszy od commitu `226c460` ten projekt jest objęty licencją **GNU General Public License v3.0**. Wcześniejsze commity i już opublikowane wydania pozostają pod licencją **Eclipse Public License 2.0 (EPL-2.0)**, której nie można wycofać.

[GNU General Public License v3.0](../../LICENSE)

Informacje o komponentach trzecich: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## Inne wersje językowe

Pełne tłumaczenia (18 języków): zobacz linki do języków na początku dokumentu lub otwórz [README.en.md](./README.en.md).
