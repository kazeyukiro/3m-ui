# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · **Bahasa Indonesia** · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Panel web manajemen server Mihomo**

> **Stabil lebih dulu:** instalasi dan pembaruan secara default hanya menggunakan rilis resmi. Pre-release harus dipilih secara eksplisit. Paket instalasi lengkap dan image Docker menyertakan Mihomo dengan versi yang dikunci; lihat [instalasi, upgrade, dan pemulihan](https://3m-ui.top/guide/install).

Ringan dan self-hosted, untuk mengelola Listener, pengguna, langganan, dan status runtime [Mihomo](https://github.com/MetaCubeX/mihomo) di Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Dokumentasi: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Situs web: [https://3m-ui.top/](https://3m-ui.top/)

---

## Fitur

| Kategori | Kemampuan |
|------|------|
| **Node** | Listener yang digerakkan oleh registry protokol: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, dan lain-lain; Reality, TLS self-signed, sertifikat batch satu klik, **pengali traffic**, validasi eksklusif antar-field transport layer |
| **Pengguna** | **Buat satu klik** (username/password/UUID acak, opsional bind ke semua node), bind node, kuota traffic, **penghitungan per node** (penagihan = aktual × pengali node), masa berlaku, batas IP / tarik langganan, mulai dari penggunaan pertama, perpanjangan / reset berkala, tag grup, penggabungan langganan eksternal, operasi batch, token langganan |
| **Langganan** | Deteksi otomatis berbasis UA untuk Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` selalu Base64), sing-box JSON; `?target=` opsional; halaman langganan HTML; tautan berbagi TUIC/HY2 menyertakan parameter klien seperti SNI dan `allow_insecure` |
| **Konfigurasi** | Generate → validasi → terapkan dipisahkan; rollback ke `config.yaml` sebelumnya jika gagal |
| **Telegram** | Alarm dan perintah manajemen (memerlukan Token + Chat ID) |
| **Multi-mesin** | Mendaftarkan panel jarak jauh, health check, sinkronisasi mirror node, penggabungan langganan; **push node lokal berdasarkan nama** ke mesin jarak jauh (dibuat nonaktif secara default, nama/port dapat diubah) |
| **Keamanan** | JWT, ganti password saat login pertama, TOTP dua langkah, **GitHub OAuth** opsional, awalan path web_path, enkripsi kredensial, CORS |
| **Operasional** | Start/stop/update core, Geo, SSL/ACME panel (termasuk wildcard DNS-01), backup, registrasi **Cloudflare WARP** satu klik (WireGuard/MASQUE YAML) |

### Catatan tambahan (batas pengguna / sertifikat / langganan)

- [Batas pengguna: IP · tarikan langganan](https://3m-ui.top/guide/users-limits)
- [Traffic per node dan pengali](https://3m-ui.top/guide/node-traffic)
- [SSL/ACME panel (HTTP-01, wildcard DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [Terapkan sertifikat node secara batch](https://3m-ui.top/guide/batch-certificate)
- [Format langganan dan target](https://3m-ui.top/guide/subscription-formats)
- [Node multi-mesin / push](https://3m-ui.top/guide/cluster)
- [Routing (langganan klien + egress server)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Frontend: **React + Ant Design + Lucide Icons** (`frontend/`), setelah di-build disematkan ke dalam satu binary Go.

Artifact rilis: **linking statis penuh** (`CGO_ENABLED=0` + modernc SQLite), tidak memerlukan `libsqlite3` sistem, kompatibel dengan glibc / musl.

Paket instalasi lengkap / Docker: `linux/amd64` · `arm64`. Binary panel mandiri tingkat lanjut juga mendukung `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`, dan memerlukan kernel yang kompatibel yang disiapkan sendiri.

---

## Instalasi Cepat

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Instalasi lengkap mendukung Linux amd64 / arm64, dan memasang panel, Mihomo pendamping, serta layanan sistem secara otomatis. Konfigurasi awal dan password admin dibuat oleh program; jika konfigurasi sudah ada, nilainya dipertahankan. Untuk arsitektur lain dapat digunakan binary mandiri dengan kernel kompatibel yang disiapkan sendiri, lihat [petunjuk instalasi lengkap](https://3m-ui.top/guide/install).

Setelah instalasi selesai:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Pembaruan:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Uninstall:

```bash
sudo 3m-ui uninstall
```

### Instalasi dengan Prompt AI

Salin prompt tersebut ke ChatGPT / Claude / Cursor atau asisten sejenis, lalu sebutkan sistem Anda (mis. Ubuntu 22.04) dan apakah Anda sudah memiliki akses SSH root. Asisten sebaiknya hanya membantu menjalankan skrip resmi, dan tidak mengubahnya menjadi perintah dari sumber yang tidak dikenal. Prompt lengkap beserta petunjuk penggunaan ada di [dokumen prompt instalasi AI](../ai-install-prompt.md).

Tanpa perlu menyalin teks panjang secara manual, gunakan `curl` untuk memuat prompt teks biasa langsung ke clipboard lalu tempel:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Prompt hanya mengarahkan ke skrip resmi satu baris di bawah ini, dan semua unduhan diverifikasi dengan `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Kanal pengujian / pre-release (opsional, bukan default) ada di [dokumen instalasi](../installation.md#ai-提示词安装).

---

## Instalasi Docker

Di VPS Linux, unduh `docker-compose.yml` dari rilis yang dipublikasikan, lalu jalankan di direktori yang sama:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Image-nya adalah `ghcr.io/kazeyukiro/3m-ui:latest`, berisi panel dan Mihomo dengan versi yang dikunci; `latest` hanya mengikuti rilis stabil. Konfigurasi, kunci acak, dan admin diinisialisasi otomatis, dan password pertama hanya ditampilkan sekali pada peluncuran pertama. Secara default menggunakan jaringan host; setelah menambah node, buka port TCP/UDP yang benar-benar digunakan. Tersedia juga `docker-compose.bridge.yml`, serta penjelasan tentang penguncian versi, konfigurasi sertifikat, dan pemulihan persistensi; lihat [dokumen instalasi](https://3m-ui.top/guide/install).

---

## Build dari Source

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Binary **Mihomo** yang kompatibel harus sudah terpasang di mesin, dan path-nya harus ditunjuk di konfigurasi.

---

## Catatan Konfigurasi

Data dan konfigurasi default kurang lebih berada di:

| Path | Kegunaan |
|------|------|
| `/etc/3m-ui/config.yaml` | Konfigurasi panel |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Sertifikat self-signed node (**harap di-backup bersama database**) |
| `/var/lib/3m-ui/mihomo/` | Direktori kerja Mihomo dan `config.yaml` |

Gunakan nilai acak yang independen (≥ 32 byte) untuk kunci JWT / kredensial, jangan gunakan contoh dari dokumentasi.

Port panel kustom: `3m-ui config port <port>` atau edit konfigurasi lalu restart layanan.

Node baru secara default diisi port listening acak antara 10000～60000 (sampling ringan dan menghindari port node yang sudah dimuat; mendukung rentang terbalik), dapat diubah manual atau dengan mengklik "port acak".

---

## Diagram Arsitektur

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

## Rekomendasi Backup

Minimal backup:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` panel dan direktori konfigurasi Mihomo

Jika hanya memperbarui binary, sertifikat sedapat mungkin di-hydrate dari disk; jika direktori hilang, sertifikat akan di-self-sign ulang dan klien perlu menarik ulang langganan.

---

## Ucapan Terima Kasih

Terima kasih khusus kepada proyek open-source dan komunitas berikut:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — mesin proxy inti dan model konfigurasi Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — penerbitan / penerapan / perpanjangan sertifikat di menu SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — contoh konfigurasi Listener dan referensi protokol
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — referensi interaksi panel dan ide operasional
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP backend
- [GORM](https://github.com/go-gorm/gorm) — ORM database
- [React](https://github.com/facebook/react) — fondasi frontend
- [Ant Design](https://github.com/ant-design/ant-design) — komponen UI
- [Lucide](https://github.com/lucide-icons/lucide) — ikon (`lucide-react`, diadaptasi melalui `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — referensi konfigurasi routing Mihomo dan ide aturan ringkas
- [echs-top/proxy](https://github.com/echs-top/proxy) — referensi aturan Mihomo dan skema proxy-group
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — referensi rule set dan template routing mihomo
- [Zustand](https://github.com/pmndrs/zustand) — manajemen state frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js, dan seluruh komunitas open-source

Kontributor: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Lisensi

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Sejak commit `226c460`, proyek ini menggunakan **GNU General Public License v3.0**. Commit sebelum itu dan rilis yang sudah dipublikasikan tetap tunduk pada **Eclipse Public License 2.0 (EPL-2.0)**, yang tidak dapat dicabut.

[GNU General Public License v3.0](../../LICENSE)

Komponen pihak ketiga: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## Bahasa Inggris & bahasa lainnya

Terjemahan lengkap (18 bahasa): lihat tautan bahasa di bagian atas, atau buka [README.en.md](./README.en.md).
