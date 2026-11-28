# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · **العربية** · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**لوحة تحكم ويب لإدارة خادم Mihomo**

> **الأولوية للإصدار المستقر:** يعتمد التثبيت والتحديث افتراضيًا على الإصدارات الرسمية فقط. يتطلب استخدام الإصدارات التجريبية (pre-release) اختيارًا صريحًا. تحتوي حزم التثبيت الكاملة وصور Docker على إصدار مثبّت من Mihomo؛ راجع [التثبيت والترقية والاستعادة](https://3m-ui.top/guide/install).

خفيفة الوزن ومستضافة ذاتيًا، لإدارة Listener والمستخدمين والاشتراكات وحالة تشغيل [Mihomo](https://github.com/MetaCubeX/mihomo) على Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> الوثائق: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · الموقع الرسمي: [https://3m-ui.top/](https://3m-ui.top/)

---

## المميزات

| الفئة | القدرات |
|------|------|
| **العقد** | Listener مدفوعة بسجل البروتوكولات: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC وغيرها؛ Reality، TLS موقّع ذاتيًا، إصدار شهادات دفعية بنقرة واحدة، **مضاعف الترافيك**، التحقق من تنافر حقول طبقة النقل |
| **المستخدمون** | **إنشاء بنقرة واحدة** (اسم مستخدم / كلمة مرور / UUID عشوائية، مع خيار الربط بكل العقد)، ربط العقد، حدود الترافيك، **القياس لكل عقدة** (الاحتساب = الفعلي × مضاعف العقدة)، تاريخ الانتهاء، قيود IP / سحب الاشتراك، الاحتساب من أول استخدام، التجديد / إعادة التعيين الدوري، تسميات المجموعات، دمج الاشتراكات الخارجية، عمليات دفعية، رمز الاشتراك Token |
| **الاشتراكات** | تمييز تلقائي عبر UA لـ Clash/Mihomo YAML و v2ray Base64 (`?target=v2ray` دائمًا Base64) و sing-box JSON؛ `?target=` اختياري؛ صفحة اشتراك HTML؛ روابط مشاركة TUIC/HY2 تتضمن SNI و`allow_insecure` وغيرها من معاملات العميل |
| **التكوين** | فصل التوليد → التحقق → التطبيق؛ التراجع إلى `config.yaml` السابق عند الفشل |
| **Telegram** | تنبيهات وأوامر إدارة (تتطلب Token + Chat ID) |
| **تعدد الأجهزة** | تسجيل اللوحات البعيدة، فحوصات الصحة، مزامنة صور العقد، دمج الاشتراكات؛ **دفع عقد الجهاز المحلي بالاسم** إلى الجهاز البعيد (مُنشأة معطلة افتراضيًا، ويمكن تغيير الاسم / المنفذ) |
| **الأمان** | JWT، تغيير كلمة المرور عند أول تسجيل دخول، تحقق بخطوتين TOTP، **GitHub OAuth** اختياري، بادئة مسار web_path، تشفير بيانات الاعتماد، CORS |
| **العمليات** | إيقاف / تشغيل / تحديث النواة، Geo، SSL/ACME للوحة (بما في ذلك DNS-01 لأحرف البدل)، النسخ الاحتياطي، التسجيل بنقرة واحدة في **Cloudflare WARP** (WireGuard/MASQUE YAML) |

### ملاحظات إضافية (قيود المستخدمين / الشهادات / الاشتراكات)

- [قيود المستخدمين: IP · سحب الاشتراك](https://3m-ui.top/guide/users-limits)
- [الترافيك لكل عقدة والمضاعف](https://3m-ui.top/guide/node-traffic)
- [SSL/ACME للوحة (HTTP-01، أحرف البدل عبر DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [تطبيق شهادات العقد بشكل دفعي](https://3m-ui.top/guide/batch-certificate)
- [صيغ الاشتراك و target](https://3m-ui.top/guide/subscription-formats)
- [العقد متعددة الأجهزة / الدفع](https://3m-ui.top/guide/cluster)
- [التوجيه (اشتراك العميل + الخروج من الخادم)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

الواجهة الأمامية: **React + Ant Design + Lucide Icons** (`frontend/`)، ويتم تضمينها بعد البناء في ملف تنفيذي Go واحد.

مخرجات الإصدار: **ربط ثابت بالكامل** (`CGO_ENABLED=0` + modernc SQLite)، دون الحاجة إلى `libsqlite3` في النظام، ومتوافقة مع glibc / musl.

حزم التثبيت الكاملة / Docker: `linux/amd64` · `arm64`. يدعم الملف التنفيذي المستقل للوحة المتقدمة أيضًا `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`، ويلزم تجهيز نواة متوافقة بنفسك.

---

## التثبيت السريع

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

يدعم التثبيت الكامل نظام Linux بمعماريتَي amd64 / arm64، ويقوم تلقائيًا بتثبيت اللوحة و Mihomo المصاحب وخدمات النظام. يتم توليد التكوين الأولي وكلمة مرور المدير بواسطة البرنامج؛ وعند وجود التكوين مسبقًا تُحفظ القيم الأصلية. للمعماريات الأخرى يمكن استخدام الملف التنفيذي المستقل مع نواة متوافقة تجهزها بنفسك، راجع [تعليمات التثبيت المفصلة](https://3m-ui.top/guide/install).

بعد اكتمال التثبيت:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

التحديث:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

الإزالة:

```bash
sudo 3m-ui uninstall
```

### التثبيت باستخدام مطالبة الذكاء الاصطناعي

انسخ المطالبة إلى ChatGPT / Claude / Cursor أو ما شابهها، واذكر نظامك (مثل Ubuntu 22.04) وما إذا كان لديك وصول SSH بصلاحيات root. يجب أن يقتصر دور المساعد على تنفيذ السكربت الرسمي فقط، دون إعادة صياغته إلى أوامر من مصادر غير معروفة. المطالبة الكاملة وتعليمات استخدامها في [وثيقة مطالبة التثبيت بالذكاء الاصطناعي](../ai-install-prompt.md).

بدون الحاجة إلى نسخ مقاطع نصية طويلة يدويًا، استخدم `curl` لتحميل المطالبة النصية مباشرة إلى الحافظة ثم الصقها:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

توجّه المطالبة فقط إلى استخدام سكربت التثبيت الرسمي أدناه، وتتحقق جميع التنزيلات من `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

قناة الاختبار / الإصدارات التجريبية (اختيارية وليست الافتراضية) في [وثيقة التثبيت](../installation.md#ai-提示词安装).

---

## تثبيت Docker

على خادم Linux VPS، نزّل ملف `docker-compose.yml` من الإصدار المنشور، ثم نفّذ في نفس المجلد:

```bash
docker compose up -d
docker compose logs 3m-ui
```

الصورة هي `ghcr.io/kazeyukiro/3m-ui:latest`، وتحتوي على اللوحة وإصدار مثبّت من Mihomo؛ و`latest` يتبع الإصدار المستقر فقط. تتم تهيئة التكوين والمفاتيح العشوائية ومدير النظام تلقائيًا، وتظهر كلمة المرور الأولى مرة واحدة فقط عند أول تشغيل. شبكة host هي الافتراضية؛ بعد إضافة العقد اسمح بمنافذ TCP/UDP المستخدمة فعليًا. يتوفر أيضًا `docker-compose.bridge.yml`، إلى جانب تثبيت الإصدار وتهيئة الشهادات وتعليمات الاستعادة الدائمة، راجع [وثيقة التثبيت](https://3m-ui.top/guide/install).

---

## البناء من المصدر

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

يلزم وجود ملف تنفيذي **Mihomo** متوافق مثبّت على الجهاز، مع تحديد مساره في التكوين.

---

## ملاحظات التكوين

تقع بيانات وتكوينات النظام الافتراضية تقريبًا في:

| المسار | الاستخدام |
|------|------|
| `/etc/3m-ui/config.yaml` | تكوين اللوحة |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | الشهادات الموقّعة ذاتيًا للعقد (**يرجى نسخها احتياطيًا مع قاعدة البيانات**) |
| `/var/lib/3m-ui/mihomo/` | مجلد عمل Mihomo وملف `config.yaml` |

استخدم لمفاتيح JWT وبيانات الاعتماد قيمًا عشوائية مستقلة (≥ 32 بايت)، ولا تستخدم أمثلة الوثائق.

منفذ اللوحة المخصص: `3m-ui config port <port>` أو عدّل التكوين ثم أعد تشغيل الخدمة.

تملأ العقد الجديدة افتراضيًا منفذ استماع عشوائيًا بين 10000 و60000 (مع أخذ عينات خفيفة وتجنب منافذ العقد المحمّلة؛ ويدعم النطاق بترتيب عكسي)، ويمكن تعديله يدويًا أو بالنقر على «منفذ عشوائي».

---

## مخطط المعمارية

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

## توصيات النسخ الاحتياطي

قم بعمل نسخ احتياطي على الأقل من:

1. `3m-ui.db`
2. `listener-certs/`
3. ملف `config.yaml` للوحة ومجلد تكوين Mihomo

عند تحديث الملف التنفيذي فقط، تُستعاد الشهادات من القرص قدر الإمكان (hydrate)؛ وإذا فُقد المجلد تُوقّع الشهادات ذاتيًا من جديد، وسيحتاج العملاء إلى إعادة سحب الاشتراك.

---

## شكر وتقدير

شكر خاص للمشاريع مفتوحة المصدر والمجتمعات التالية:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — محرك البروكسي الأساسي ونموذج تكوين Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — إصدار / نشر / تجديد الشهادات في قائمة SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — أمثلة تكوين Listener ومرجع البروتوكولات
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — مرجع لتفاعلات اللوحة وأفكار العمليات
- [Gin](https://github.com/gin-gonic/gin) — إطار HTTP للواجهة الخلفية
- [GORM](https://github.com/go-gorm/gorm) — ORM لقاعدة البيانات
- [React](https://github.com/facebook/react) — أساس الواجهة الأمامية
- [Ant Design](https://github.com/ant-design/ant-design) — مكونات الواجهة
- [Lucide](https://github.com/lucide-icons/lucide) — الأيقونات (`lucide-react`، عبر `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — مرجع لتكوين توجيه Mihomo وأفكار القواعد المبسطة
- [echs-top/proxy](https://github.com/echs-top/proxy) — مرجع لقواعد Mihomo ومخططات مجموعات البروكسي
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — مرجع لمجموعات القواعد وقوالب توجيه mihomo
- [Zustand](https://github.com/pmndrs/zustand) — إدارة حالة الواجهة الأمامية
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go و Node.js ومجتمع المصادر المفتوحة بأكمله

المساهمون: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## الترخيص

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

يعتمد هذا المشروع منذ commit `226c460` ترخيص **GNU General Public License v3.0**. أما الالتزامات السابقة والإصدارات التي نُشرت قبل ذلك فتظل خاضعة لـ **Eclipse Public License 2.0（EPL-2.0）**، وهذا الترخيص غير قابل للإلغاء.

[GNU General Public License v3.0](../../LICENSE)

المكونات الخارجية: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## الإنجليزية واللغات الأخرى

الترجمات الكاملة (18 لغة): راجع روابط اللغات في الأعلى، أو افتح [README.en.md](./README.en.md).
