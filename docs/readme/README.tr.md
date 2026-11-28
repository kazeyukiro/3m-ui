# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · **Türkçe** · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo sunucusu için web yönetim paneli**

> **Önce kararlı sürüm:** kurulum ve güncelleme varsayılan olarak yalnızca resmi Release sürümlerini kullanır. Ön sürümler (pre-release) için açıkça seçim yapmak gerekir. Tam kurulum paketleri ve Docker imajları sabitlenmiş bir Mihomo sürümü içerir; ayrıntılar için [kurulum, yükseltme ve geri yükleme](https://3m-ui.top/guide/install) bakın.

Hafif ve self-hosted; Linux üzerinde [Mihomo](https://github.com/MetaCubeX/mihomo) Listener'larını, kullanıcıları, abonelikleri ve çalışma durumunu yönetmek için.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Belgeler: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Web sitesi: [https://3m-ui.top/](https://3m-ui.top/)

---

## Özellikler

| Kategori | Yetenekler |
|------|------|
| **Düğümler** | Protokol kayıt defteriyle yönetilen Listener: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC vb.; Reality, TLS self-signed, tek tıkla toplu sertifika, **trafik çarpanı**, taşıma katmanı alanları için birbirini dışlayan doğrulama |
| **Kullanıcılar** | **Tek tıkla oluşturma** (rastgele kullanıcı adı/şifre/UUID, isteğe bağlı olarak tüm düğümlere bağlama), düğüm bağlama, trafik kotası, **düğüm başına ölçüm** (faturalandırma = gerçek × düğüm çarpanı), son kullanma tarihi, IP / abonelik çekme sınırı, ilk kullanımdan itibaren başlatma, periyodik yenileme / sıfırlama, grup etiketleri, harici abonelik birleştirme, toplu işlemler, abonelik Token'ı |
| **Abonelikler** | UA tabanlı otomatik algılama: Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` her zaman Base64), sing-box JSON; isteğe bağlı `?target=`; HTML abonelik sayfası; TUIC/HY2 paylaşım bağlantılarında SNI ve `allow_insecure` gibi istemci parametreleri |
| **Yapılandırma** | Oluştur → doğrula → uygula ayrımı; hata durumunda önceki `config.yaml` sürümüne geri alma |
| **Telegram** | Uyarılar ve yönetim komutları (Token + Chat ID gerekir) |
| **Çoklu makine** | Uzak panelleri kaydetme, sağlık kontrolü, düğüm ayna senkronizasyonu, birleştirilmiş abonelikler; **ada göre yerel düğümleri uzağa push etme** (varsayılan olarak devre dışı oluşturulur, ad/port değiştirilebilir) |
| **Güvenlik** | JWT, ilk girişte şifre değiştirme, TOTP iki adımlı doğrulama, isteğe bağlı **GitHub OAuth**, web_path yol ön eki, kimlik bilgilerinin şifrelenmesi, CORS |
| **Operasyon** | Çekirdek başlatma/durdurma/güncelleme, Geo, panel SSL/ACME (DNS-01 wildcard dahil), yedekleme, **Cloudflare WARP** tek tıkla kayıt (WireGuard/MASQUE YAML) |

### Ek açıklamalar (kullanıcı sınırları / sertifikalar / abonelikler)

- [Kullanıcı sınırları: IP · abonelik çekme](https://3m-ui.top/guide/users-limits)
- [Düğüm başına trafik ve çarpan](https://3m-ui.top/guide/node-traffic)
- [Panel SSL / ACME (HTTP-01, DNS-01 wildcard)](https://3m-ui.top/guide/ssl-cert)
- [Düğüm sertifikalarını toplu uygulama](https://3m-ui.top/guide/batch-certificate)
- [Abonelik biçimleri ve target](https://3m-ui.top/guide/subscription-formats)
- [Çoklu makine düğümleri / push](https://3m-ui.top/guide/cluster)
- [Yönlendirme (istemci aboneliği + sunucu çıkışı)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`); derlendikten sonra tek bir Go ikili dosyasına gömülür.

Sürüm çıktıları: **tamamen statik bağlama** (`CGO_ENABLED=0` + modernc SQLite), sistem `libsqlite3` gerektirmez, glibc / musl ile uyumludur.

Tam kurulum paketi / Docker: `linux/amd64` · `arm64`. Gelişmiş bağımsız panel ikili dosyası ayrıca `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` destekler ve uyumlu bir çekirdeğin kendiniz hazırlanmasını gerektirir.

---

## Hızlı Kurulum

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Tam kurulum Linux amd64 / arm64 destekler; paneli, eşlik eden Mihomo'yu ve sistem servisini otomatik olarak kurar. İlk yapılandırma ve yönetici şifresi program tarafından oluşturulur; yapılandırma zaten mevcutsa mevcut değerler korunur. Diğer mimariler için bağımsız ikili dosya ve kendinizin hazırladığı uyumlu çekirdek kullanılabilir, bkz. [ayrıntılı kurulum talimatları](https://3m-ui.top/guide/install).

Kurulum tamamlandıktan sonra:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Güncelleme:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Kaldırma:

```bash
sudo 3m-ui uninstall
```

### AI İstemi ile Kurulum

İstemi ChatGPT / Claude / Cursor gibi asistanlara kopyalayın ve sisteminizi (örn. Ubuntu 22.04) ile root SSH erişiminiz olup olmadığını belirtin. Asistan yalnızca resmi betiği çalıştırmaya yardımcı olmalı, onu bilinmeyen kaynaklı komutlara dönüştürmemelidir. İstemin tamamı ve kullanım talimatları [AI istemi kurulum belgesinde](../ai-install-prompt.md) yer alır.

Uzun metinleri elle kopyalamanıza gerek yok; `curl` ile düz metin istemini doğrudan panoya yükleyip yapıştırın:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

İstem yalnızca aşağıdaki resmi tek satırlık betiği kullanmaya yönlendirir ve tüm indirmeler `SHA256SUMS` ile doğrulanır:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Test / ön sürüm kanalı (isteğe bağlı, varsayılan değil) için [kurulum belgesine](../installation.md#ai-提示词安装) bakın.

---

## Docker Kurulumu

Linux VPS üzerinde yayınlanan sürümdeki `docker-compose.yml` dosyasını indirin ve aynı dizinde çalıştırın:

```bash
docker compose up -d
docker compose logs 3m-ui
```

İmaj `ghcr.io/kazeyukiro/3m-ui:latest` olup paneli ve sabitlenmiş sürümde Mihomo'yu içerir; `latest` yalnızca kararlı sürümü takip eder. Yapılandırma, rastgele anahtarlar ve yönetici otomatik olarak başlatılır; ilk şifre yalnızca ilk açılışta bir kez gösterilir. Varsayılan olarak host ağı kullanılır; yeni düğüm ekledikten sonra gerçekten kullanılan TCP/UDP portlarını açın. Ayrıca `docker-compose.bridge.yml`, sürüm sabitleme, sertifika yapılandırması ve kalıcı geri yükleme açıklamaları için [kurulum belgesine](https://3m-ui.top/guide/install) bakın.

---

## Kaynaktan Derleme

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Makinede uyumlu bir **Mihomo** ikili dosyasının kurulu olması ve yapılandırmada yolunun gösterilmesi gerekir.

---

## Yapılandırma Notları

Varsayılan veri ve yapılandırma dosyaları kabaca şu konumlardadır:

| Yol | Kullanım |
|------|------|
| `/etc/3m-ui/config.yaml` | Panel yapılandırması |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Düğüm self-signed sertifikaları (**lütfen veritabanıyla birlikte yedekleyin**) |
| `/var/lib/3m-ui/mihomo/` | Mihomo çalışma dizini ve `config.yaml` |

JWT / kimlik bilgisi anahtarları için bağımsız rastgele değerler (≥ 32 bayt) kullanın, belgelerdeki örnekleri kullanmayın.

Özel panel portu: `3m-ui config port <port>` veya yapılandırmayı düzenleyip servisi yeniden başlatın.

Yeni düğümler varsayılan olarak 10000～60000 arasında rastgele bir dinleme portuyla doldurulur (hafif örnekleme yapılır ve yüklü düğüm portlarından kaçınılır; ters sıralı aralık desteklenir), elle değiştirilebilir veya "rastgele port"a tıklanabilir.

---

## Mimari Şeması

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

## Yedekleme Önerileri

En az şunları yedekleyin:

1. `3m-ui.db`
2. `listener-certs/`
3. Panel `config.yaml` dosyası ve Mihomo yapılandırma dizini

Yalnızca ikili dosya güncellenirken sertifikalar mümkün olduğunca diskten hydrate edilir; dizin kaybolursa yeniden self-signed oluşturulur ve istemcilerin aboneliği yeniden çekmesi gerekir.

---

## Teşekkürler

Aşağıdaki açık kaynak projelere ve topluluklara özel teşekkürler:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — çekirdek proxy motoru ve Listener yapılandırma modeli
- [acme.sh](https://github.com/acmesh-official/acme.sh) — SSH menüsünde sertifika alma / dağıtma / yenileme
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener yapılandırma örnekleri ve protokol referansı
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — panel etkileşimi ve operasyon fikirleri referansı
- [Gin](https://github.com/gin-gonic/gin) — backend HTTP çerçevesi
- [GORM](https://github.com/go-gorm/gorm) — veritabanı ORM
- [React](https://github.com/facebook/react) — frontend temeli
- [Ant Design](https://github.com/ant-design/ant-design) — UI bileşenleri
- [Lucide](https://github.com/lucide-icons/lucide) — ikonlar (`lucide-react`, `frontend/src/icons.tsx` üzerinden uyarlanmış)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo yönlendirme yapılandırması ve sade kural fikirleri referansı
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo kuralları ve proxy-grup şemaları referansı
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — kural setleri ve mihomo yönlendirme şablonları referansı
- [Zustand](https://github.com/pmndrs/zustand) — frontend durum yönetimi
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js ve tüm açık kaynak topluluğu

Katkıda bulunanlar: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Lisans

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Bu proje `226c460` commit'inden itibaren **GNU General Public License v3.0** altında lisanslanmıştır. Önceki commit'ler ve daha önce yayınlanmış sürümler **Eclipse Public License 2.0 (EPL-2.0)** kapsamında kalmayı sürdürür ve bu lisans geri alınamaz.

[GNU General Public License v3.0](../../LICENSE)

Üçüncü taraf bileşenler: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## İngilizce ve diğer diller

Tam çeviriler (18 dil): üstteki dil bağlantılarına bakın veya [README.en.md](./README.en.md) dosyasını açın.
