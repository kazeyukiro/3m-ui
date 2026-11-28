# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · **ไทย** · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**แผงควบคุมเว็บสำหรับจัดการเซิร์ฟเวอร์ Mihomo**

> **ให้ความสำคัญกับรุ่นเสถียร:** การติดตั้งและอัปเดตตามค่าเริ่มต้นใช้เฉพาะ Release ทางการ ต้องเลือกใช้รุ่นก่อนปล่อย (pre-release) ด้วยตนเอง แพ็กเกจติดตั้งแบบสมบูรณ์และอิมเมจ Docker จะรวม Mihomo เวอร์ชันที่ตรึงไว้ ดูรายละเอียดได้ที่ [การติดตั้ง การอัปเกรด และการกู้คืน](https://3m-ui.top/guide/install)

น้ำหนักเบา แบบ self-hosted ใช้จัดการ Listener ผู้ใช้ การสมัครสมาชิก และสถานะการทำงานของ [Mihomo](https://github.com/MetaCubeX/mihomo) บน Linux

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> เอกสาร: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · เว็บไซต์: [https://3m-ui.top/](https://3m-ui.top/)

---

## คุณสมบัติ

| หมวด | ความสามารถ |
|------|------|
| **โหนด** | Listener ที่ขับเคลื่อนด้วยรีจิสทรีโปรโตคอล: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC ฯลฯ; Reality, ใบรับรองแบบ self-signed TLS, ออกใบรับรองเป็นชุดด้วยคลิกเดียว, **ตัวคูณทราฟฟิก**, ตรวจสอบความขัดแย้งของฟิลด์ชั้นการขนส่ง |
| **ผู้ใช้** | **สร้างด้วยคลิกเดียว**(ชื่อผู้ใช้/รหัสผ่าน/UUID แบบสุ่ม เลือกผูกกับทุกโหนดได้), ผูกโหนด, โควตาทราฟฟิก, **วัดแยกตามโหนด**(คิดเงิน = ปริมาณจริง × ตัวคูณโหนด), วันหมดอายุ, จำกัด IP/การดึง subscription, นับจากการใช้งานครั้งแรก, ต่ออายุ/รีเซ็ตตามรอบ, แท็กกลุ่ม, รวม subscription ภายนอก, การทำงานเป็นชุด, Token subscription |
| **การสมัครสมาชิก** | ระบุชนิดอัตโนมัติจาก UA: Clash/Mihomo YAML, v2ray Base64(`?target=v2ray` เป็น Base64 เสมอ), sing-box JSON; `?target=` ที่เลือกได้; หน้า subscription แบบ HTML; ลิงก์แชร์ TUIC/HY2 ที่มีพารามิเตอร์ฝั่งไคลเอนต์ เช่น SNI/`allow_insecure` |
| **การกำหนดค่า** | แยกขั้นตอน สร้าง → ตรวจสอบ → นำไปใช้; เมื่อล้มเหลวจะย้อนกลับไปใช้ `config.yaml` ก่อนหน้า |
| **Telegram** | การแจ้งเตือนและคำสั่งจัดการ (ต้องใช้ Token + Chat ID) |
| **หลายเครื่อง** | ลงทะเบียนแผงระยะไกล, ตรวจสอบสุขภาพ, ซิงก์มิเรอร์โหนด, รวม subscription; **เลือกโหนดในเครื่องตามชื่อเพื่อพุชไปยังระยะไกล**(ปิดใช้งานตามค่าเริ่มต้น เปลี่ยนชื่อ/พอร์ตได้) |
| **ความปลอดภัย** | JWT, เปลี่ยนรหัสผ่านเมื่อเข้าสู่ระบบครั้งแรก, การยืนยันสองขั้นตอน TOTP, **GitHub OAuth** ที่เลือกได้, คำนำหน้าเส้นทาง web_path, เข้ารหัสข้อมูลรับรอง, CORS |
| **การดูแลระบบ** | เริ่ม/หยุด/อัปเดตคอร์, Geo, SSL/ACME ของแผง (รวม wildcard แบบ DNS-01), สำรองข้อมูล, ลงทะเบียน **Cloudflare WARP** ด้วยคลิกเดียว (WireGuard/MASQUE YAML) |

### หมายเหตุเพิ่มเติม (ข้อจำกัดผู้ใช้ / ใบรับรอง / การสมัครสมาชิก)

- [ข้อจำกัดผู้ใช้: IP · การดึง subscription](https://3m-ui.top/guide/users-limits)
- [ทราฟฟิกตามโหนดและตัวคูณ](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME ของแผง (HTTP-01, wildcard แบบ DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [ใช้ใบรับรองโหนดเป็นชุด](https://3m-ui.top/guide/batch-certificate)
- [รูปแบบ subscription และ target](https://3m-ui.top/guide/subscription-formats)
- [โหนดหลายเครื่อง / การพุช](https://3m-ui.top/guide/cluster)
- [การจัดเส้นทาง (subscription ฝั่งไคลเอนต์ + ขาออกฝั่งเซิร์ฟเวอร์)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

ส่วนหน้า: **React + Ant Design + Lucide Icons**(`frontend/`) หลังจาก build จะถูกฝังไว้ในไฟล์ไบนารี Go เดียว

ผลงานที่ปล่อยออกมา: **การลิงก์แบบสถิตสมบูรณ์**(`CGO_ENABLED=0` + modernc SQLite) ไม่ต้องพึ่ง `libsqlite3` ของระบบ และเข้ากันได้ทั้ง glibc / musl

แพ็กเกจติดตั้งแบบสมบูรณ์ / Docker: `linux/amd64` · `arm64` ไบนารีแผงแบบสแตนด์อโลนขั้นสูงยังรองรับเพิ่มเติมคือ `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` โดยต้องเตรียมเคอร์เนลที่เข้ากันได้เอง

---

## ติดตั้งอย่างรวดเร็ว

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

การติดตั้งแบบสมบูรณ์รองรับ Linux amd64 / arm64 ติดตั้งแผง Mihomo ที่มาด้วยกัน และบริการของระบบโดยอัตโนมัติ การกำหนดค่าครั้งแรกและรหัสผ่านผู้ดูแลระบบถูกสร้างโดยโปรแกรมอย่างเป็นหนึ่งเดียว หากมีการกำหนดค่าอยู่แล้ว ค่าเดิมจะถูกเก็บไว้ สถาปัตยกรรมอื่นสามารถใช้ไบนารีแบบสแตนด์อโลนและเคอร์เนลที่เตรียมเองได้ ดู [คำแนะนำการติดตั้งโดยละเอียด](https://3m-ui.top/guide/install)

หลังจากติดตั้งเสร็จ:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

อัปเดต:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

ถอนการติดตั้ง:

```bash
sudo 3m-ui uninstall
```

### ติดตั้งด้วยพรอมต์ AI

คัดลอกพรอมต์ไปยังผู้ช่วยอย่าง ChatGPT / Claude / Cursor และระบุระบบของคุณ (เช่น Ubuntu 22.04) รวมถึงว่ามี SSH แบบ root อยู่แล้วหรือไม่ ผู้ช่วยควรช่วยรันสคริปต์ทางการเท่านั้น และต้องไม่เขียนใหม่เป็นคำสั่งจากแหล่งที่ไม่ทราบที่มา พรอมต์ฉบับเต็มและคำแนะนำการใช้งานอยู่ที่ [เอกสารการติดตั้งด้วยพรอมต์ AI](../ai-install-prompt.md)

ไม่ต้องคัดลอกข้อความยาว ๆ ด้วยตนเอง ใช้ `curl` โหลดพรอมต์แบบข้อความธรรมดาไปยังคลิปบอร์ดโดยตรงแล้ววางได้เลย:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

พรอมต์จะนำทางให้ใช้สคริปต์ทางการแบบบรรทัดเดียวด้านล่างเท่านั้น และการดาวน์โหลดทั้งหมดจะตรวจสอบ `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

ช่องทดสอบ / ก่อนปล่อย (เลือกได้ ไม่ใช่ค่าเริ่มต้น) ดู [เอกสารการติดตั้ง](../installation.md#ai-提示词安装)

---

## ติดตั้งด้วย Docker

บน Linux VPS ให้ดาวน์โหลด `docker-compose.yml` จากรุ่นที่ปล่อย แล้วรันในไดเรกทอรีเดียวกัน:

```bash
docker compose up -d
docker compose logs 3m-ui
```

อิมเมจคือ `ghcr.io/kazeyukiro/3m-ui:latest` ซึ่งประกอบด้วยแผงและ Mihomo เวอร์ชันที่ตรึงไว้ โดย `latest` จะตามเฉพาะรุ่นเสถียร การกำหนดค่า คีย์ลับแบบสุ่ม และผู้ดูแลระบบจะถูกเตรียมโดยอัตโนมัติ รหัสผ่านเริ่มต้นจะแสดงเพียงครั้งเดียวในตอนเริ่มต้นครั้งแรก ค่าเริ่มต้นใช้เครือข่ายแบบ host หลังจากเพิ่มโหนดแล้วให้เปิดพอร์ต TCP/UDP ที่ใช้งานจริง `docker-compose.bridge.yml` การตรึงเวอร์ชัน การกำหนดค่าใบรับรอง และคำแนะนำการกู้คืนข้อมูลแบบถาวร ดูได้ที่ [เอกสารการติดตั้ง](https://3m-ui.top/guide/install)

---

## Build จากซอร์ส

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

ต้องมีไบนารี **Mihomo** ที่เข้ากันได้ติดตั้งอยู่ในเครื่อง และระบุเส้นทางของมันในการกำหนดค่า

---

## จุดสำคัญของการกำหนดค่า

ข้อมูลและการกำหนดค่าเริ่มต้นอยู่โดยประมาณที่:

| เส้นทาง | การใช้งาน |
|------|------|
| `/etc/3m-ui/config.yaml` | การกำหนดค่าของแผง |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | ใบรับรองแบบ self-signed ของโหนด (**โปรดสำรองข้อมูลร่วมกับฐานข้อมูล**) |
| `/var/lib/3m-ui/mihomo/` | ไดเรกทอรีทำงานของ Mihomo และ `config.yaml` |

คีย์ลับของ JWT / ข้อมูลรับรอง โปรดใช้ค่าสุ่มที่แยกต่างหาก (≥ 32 ไบต์) และอย่าใช้ค่าตัวอย่างในเอกสาร

ปรับแต่งพอร์ตของแผง: `3m-ui config port <port>` หรือแก้ไขการกำหนดค่าแล้วรีสตาร์ทบริการ
โหนดที่สร้างใหม่จะถูกเติมพอร์ตรับฟังแบบสุ่มระหว่าง 10000～60000 (สุ่มตัวอย่างแบบเบาและหลีกเลี่ยงพอร์ตของโหนดที่โหลดแล้ว รองรับช่วงแบบย้อนกลับ) สามารถแก้ไขด้วยตนเองหรือคลิก «พอร์ตแบบสุ่ม»

---

## แผนภาพสถาปัตยกรรม

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

## คำแนะนำการสำรองข้อมูล

อย่างน้อยต้องสำรองข้อมูล:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` ของแผงและไดเรกทอรีการกำหนดค่าของ Mihomo

เมื่ออัปเดตเฉพาะไบนารี ใบรับรองจะถูก hydrate จากดิสก์ให้มากที่สุด หากไดเรกทอรีสูญหาย จะมีการออกใบรับรองแบบ self-signed ใหม่ และไคลเอนต์ต้องดึง subscription ใหม่

---

## กิตติกรรมประกาศ

ขอขอบคุณเป็นพิเศษต่อโปรเจกต์โอเพนซอร์สและชุมชนต่อไปนี้:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — เอนจินพร็อกซีหลักและโมเดลการกำหนดค่า Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — การขอ การติดตั้ง และการต่ออายุใบรับรองในเมนู SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — ตัวอย่างการกำหนดค่า Listener และข้อมูลอ้างอิงโปรโตคอล
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — แนวทางอ้างอิงด้านการโต้ตอบและการดูแลระบบของแผง
- [Gin](https://github.com/gin-gonic/gin) — เฟรมเวิร์ก HTTP ฝั่งแบ็กเอนด์
- [GORM](https://github.com/go-gorm/gorm) — ORM สำหรับฐานข้อมูล
- [React](https://github.com/facebook/react) — พื้นฐานฝั่งฟรอนต์เอนด์
- [Ant Design](https://github.com/ant-design/ant-design) — คอมโพเนนต์ UI
- [Lucide](https://github.com/lucide-icons/lucide) — ไอคอน (`lucide-react` ปรับใช้ผ่าน `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — แนวทางอ้างอิงการกำหนดค่าการจัดเส้นทาง Mihomo และแนวคิดกฎแบบย่อ
- [echs-top/proxy](https://github.com/echs-top/proxy) — แนวทางอ้างอิงกฎของ Mihomo และรูปแบบกลุ่มนโยบาย
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — แนวทางอ้างอิงชุดกฎและเทมเพลตการจัดเส้นทาง mihomo
- [Zustand](https://github.com/pmndrs/zustand) — การจัดการสถานะฝั่งฟรอนต์เอนด์
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js และชุมชนโอเพนซอร์สทั้งหมด

ผู้มีส่วนร่วม: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## ใบอนุญาต

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

โปรเจกต์นี้ นับจาก commit `226c460` เป็นต้นไป ใช้สัญญาอนุญาต **GNU General Public License v3.0** คอมมิตก่อนหน้านี้และรุ่นที่ปล่อยออกไปแล้ว ยังคงอยู่ภายใต้ **Eclipse Public License 2.0 (EPL-2.0)** ซึ่งไม่สามารถเพิกถอนได้

[GNU General Public License v3.0](../../LICENSE)

ประกาศบุคคลที่สามดูได้ที่ [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)

---

---

## ภาษาอื่น ๆ

คำแปลฉบับสมบูรณ์ (18 ภาษา): ดูลิงก์ภาษาที่ด้านบน หรือเปิด [README.en.md](./README.en.md)
