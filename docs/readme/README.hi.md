# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · **हिन्दी** · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo सर्वर के लिए वेब प्रबंधन पैनल**

> **स्थिर संस्करण प्राथमिक:** डिफ़ॉल्ट रूप से इंस्टॉलेशन और अपडेट केवल आधिकारिक Release का उपयोग करते हैं। प्री-रिलीज़ (pre-release) के लिए स्पष्ट रूप से चयन करना आवश्यक है। पूर्ण इंस्टॉल पैकेज और Docker इमेज में निश्चित संस्करण का Mihomo शामिल है; विवरण के लिए [इंस्टॉलेशन, अपग्रेड और पुनर्स्थापना](https://3m-ui.top/guide/install) देखें।

हल्का और स्व-होस्टेड, Linux पर [Mihomo](https://github.com/MetaCubeX/mihomo) Listener, उपयोगकर्ताओं, सब्सक्रिप्शन और रनटाइम स्थिति के प्रबंधन के लिए।

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> दस्तावेज़: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · आधिकारिक साइट: [https://3m-ui.top/](https://3m-ui.top/)

---

## विशेषताएँ

| श्रेणी | क्षमताएँ |
|------|------|
| **नोड्स** | प्रोटोकॉल रजिस्ट्री द्वारा संचालित Listener: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC आदि; Reality, TLS सेल्फ़-साइन्ड, एक-क्लिक बैच प्रमाणपत्र, **ट्रैफ़िक गुणक**, ट्रांसपोर्ट लेयर फ़ील्ड की पारस्परिक-अनन्य जाँच |
| **उपयोगकर्ता** | **एक-क्लिक निर्माण** (यादृच्छिक उपयोगकर्ता नाम / पासवर्ड / UUID, वैकल्पिक रूप से सभी नोड्स से बाँधना), नोड बाइंडिंग, ट्रैफ़िक सीमा, **प्रति-नोड मापन** (बिलिंग = वास्तविक × नोड गुणक), समाप्ति तिथि, IP / सब्सक्रिप्शन पुल सीमा, प्रथम उपयोग से गणना, आवधिक नवीनीकरण / रीसेट, समूह टैग, बाहरी सब्सक्रिप्शन मर्ज, बैच ऑपरेशन, सब्सक्रिप्शन Token |
| **सब्सक्रिप्शन** | UA के आधार पर Clash/Mihomo YAML, v2ray Base64 (`?target=v2ray` हमेशा Base64), sing-box JSON की स्वचालित पहचान; वैकल्पिक `?target=`; HTML सब्सक्रिप्शन पृष्ठ; TUIC/HY2 शेयर लिंक में SNI और `allow_insecure` जैसे क्लाइंट पैरामीटर |
| **कॉन्फ़िगरेशन** | जेनरेट → सत्यापन → लागू, अलग-अलग चरण; विफलता पर पिछली `config.yaml` पर रोलबैक |
| **Telegram** | अलर्ट और प्रबंधन कमांड (Token + Chat ID आवश्यक) |
| **मल्टी-मशीन** | रिमोट पैनल पंजीकरण, हेल्थ चेक, नोड मिरर सिंक, मर्ज्ड सब्सक्रिप्शन; **नाम के अनुसार स्थानीय नोड्स को रिमोट पर पुश** करना (डिफ़ॉल्ट रूप से अक्षम, नाम/पोर्ट बदला जा सकता है) |
| **सुरक्षा** | JWT, प्रथम लॉगिन पर पासवर्ड परिवर्तन, TOTP दो-चरणीय सत्यापन, वैकल्पिक **GitHub OAuth**, web_path पथ उपसर्ग, क्रेडेंशियल एन्क्रिप्शन, CORS |
| **संचालन** | कोर स्टार्ट / स्टॉप / अपडेट, Geo, पैनल SSL/ACME (DNS-01 वाइल्डकार्ड सहित), बैकअप, **Cloudflare WARP** एक-क्लिक पंजीकरण (WireGuard/MASQUE YAML) |

### अतिरिक्त जानकारी (उपयोगकर्ता सीमाएँ / प्रमाणपत्र / सब्सक्रिप्शन)

- [उपयोगकर्ता सीमाएँ: IP · सब्सक्रिप्शन पुल](https://3m-ui.top/guide/users-limits)
- [प्रति-नोड ट्रैफ़िक और गुणक](https://3m-ui.top/guide/node-traffic)
- [पैनल SSL / ACME (HTTP-01, DNS-01 वाइल्डकार्ड)](https://3m-ui.top/guide/ssl-cert)
- [नोड प्रमाणपत्र बैच में लागू करना](https://3m-ui.top/guide/batch-certificate)
- [सब्सक्रिप्शन प्रारूप और target](https://3m-ui.top/guide/subscription-formats)
- [मल्टी-मशीन नोड्स / पुश](https://3m-ui.top/guide/cluster)
- [रूटिंग (क्लाइंट सब्सक्रिप्शन + सर्वर आउटबाउंड)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

फ़्रंटएंड: **React + Ant Design + Lucide Icons** (`frontend/`), बिल्ड के बाद एकल Go बाइनरी में एम्बेड किया जाता है।

रिलीज़ आर्टिफैक्ट: **शुद्ध स्टैटिक लिंकिंग** (`CGO_ENABLED=0` + modernc SQLite), सिस्टम `libsqlite3` की आवश्यकता नहीं, glibc / musl के साथ संगत।

पूर्ण इंस्टॉल पैकेज / Docker: `linux/amd64` · `arm64`. एडवांस्ड स्टैंडअलोन पैनल बाइनरी अतिरिक्त रूप से `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` का समर्थन करता है, जिसके लिए संगत कर्नेल स्वयं तैयार करना होगा।

---

## त्वरित इंस्टॉलेशन

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

पूर्ण इंस्टॉलेशन Linux amd64 / arm64 का समर्थन करता है, और स्वचालित रूप से पैनल, संगत Mihomo और सिस्टम सेवा इंस्टॉल करता है। प्रारंभिक कॉन्फ़िगरेशन और व्यवस्थापक (admin) पासवर्ड प्रोग्राम द्वारा स्वयं उत्पन्न किए जाते हैं; कॉन्फ़िगरेशन पहले से मौजूद होने पर मूल मान बने रहते हैं। अन्य आर्किटेक्चर के लिए स्टैंडअलोन बाइनरी और स्वयं तैयार किए गए संगत कर्नेल का उपयोग किया जा सकता है, [विस्तृत इंस्टॉलेशन निर्देश](https://3m-ui.top/guide/install) देखें।

इंस्टॉलेशन पूर्ण होने के बाद:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

अपडेट:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

अनइंस्टॉल:

```bash
sudo 3m-ui uninstall
```

### AI प्रॉम्प्ट के साथ इंस्टॉलेशन

प्रॉम्प्ट को ChatGPT / Claude / Cursor आदि सहायकों में कॉपी करें, और अपना सिस्टम (जैसे Ubuntu 22.04) तथा root SSH उपलब्ध होने की जानकारी दें। सहायक को केवल आधिकारिक स्क्रिप्ट के निष्पादन में सहायता करनी चाहिए, उसे अज्ञात स्रोत की कमांड में नहीं बदलना चाहिए। पूर्ण प्रॉम्प्ट और उपयोग निर्देश [AI प्रॉम्प्ट इंस्टॉलेशन दस्तावेज़](../ai-install-prompt.md) में देखें।

बड़े टेक्स्ट को मैन्युअल रूप से कॉपी किए बिना, `curl` से सादे-टेक्स्ट प्रॉम्प्ट को सीधे क्लिपबोर्ड में लोड करके पेस्ट करें:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

प्रॉम्प्ट केवल नीचे दी गई आधिकारिक वन-लाइनर स्क्रिप्ट का उपयोग करता है, और सभी डाउनलोड `SHA256SUMS` से सत्यापित होते हैं:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

परीक्षण / प्री-रिलीज़ चैनल (वैकल्पिक, डिफ़ॉल्ट नहीं) के लिए [इंस्टॉलेशन दस्तावेज़](../installation.md#ai-提示词安装) देखें।

---

## Docker इंस्टॉलेशन

Linux VPS पर रिलीज़ में मौजूद `docker-compose.yml` डाउनलोड करें, और उसी डायरेक्टरी में निष्पादित करें:

```bash
docker compose up -d
docker compose logs 3m-ui
```

इमेज `ghcr.io/kazeyukiro/3m-ui:latest` है, जिसमें पैनल और निश्चित संस्करण का Mihomo शामिल है; `latest` केवल स्थिर संस्करण का अनुसरण करता है। कॉन्फ़िगरेशन, यादृच्छिक कुंजियाँ और व्यवस्थापक स्वचालित रूप से इनिशियलाइज़ होते हैं, और प्रथम पासवर्ड केवल पहले स्टार्ट पर एक बार प्रदर्शित होता है। डिफ़ॉल्ट रूप से host नेटवर्क; नए नोड जोड़ने के बाद वास्तव में उपयोग किए जाने वाले TCP/UDP पोर्ट खोलें। इसके अलावा `docker-compose.bridge.yml`, संस्करण पिनिंग, प्रमाणपत्र कॉन्फ़िगरेशन और परसिस्टेंट पुनर्स्थापना निर्देशों के लिए [इंस्टॉलेशन दस्तावेज़](https://3m-ui.top/guide/install) देखें।

---

## स्रोत से बिल्ड

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

मशीन पर संगत **Mihomo** बाइनरी इंस्टॉल होना आवश्यक है, और कॉन्फ़िगरेशन में उसके पथ की ओर संकेत करना होगा।

---

## कॉन्फ़िगरेशन के मुख्य बिंदु

डिफ़ॉल्ट डेटा और कॉन्फ़िगरेशन लगभग इन स्थानों पर होते हैं:

| पथ | उपयोग |
|------|------|
| `/etc/3m-ui/config.yaml` | पैनल कॉन्फ़िगरेशन |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | नोड सेल्फ़-साइन्ड प्रमाणपत्र (**कृपया डेटाबेस के साथ बैकअप लें**) |
| `/var/lib/3m-ui/mihomo/` | Mihomo कार्यशील डायरेक्टरी और `config.yaml` |

JWT / क्रेडेंशियल कुंजियों के लिए स्वतंत्र यादृच्छिक मान (≥ 32 बाइट्स) उपयोग करें, दस्तावेज़ के उदाहरणों का उपयोग न करें।

कस्टम पैनल पोर्ट: `3m-ui config port <port>` या कॉन्फ़िगरेशन संपादित करके सेवा पुनः आरंभ करें।

नए नोड्स में डिफ़ॉल्ट रूप से 10000～60000 के बीच एक यादृच्छिक लिसनिंग पोर्ट भरा जाता है (हल्का सैंपलिंग और पहले से लोडेड नोड पोर्ट से बचाव; रिवर्स-ऑर्डर रेंज समर्थित), जिसे मैन्युअल रूप से बदला जा सकता है या "यादृच्छिक पोर्ट" पर क्लिक किया जा सकता है।

---

## आर्किटेक्चर अवलोकन

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

## बैकअप सुझाव

न्यूनतम रूप से इनका बैकअप लें:

1. `3m-ui.db`
2. `listener-certs/`
3. पैनल की `config.yaml` और Mihomo कॉन्फ़िगरेशन डायरेक्टरी

केवल बाइनरी अपडेट करते समय, प्रमाणपत्रों को डिस्क से यथासंभव hydrate किया जाता है; यदि डायरेक्टरी खो जाए तो पुनः सेल्फ़-साइन्ड किए जाते हैं, और क्लाइंट को सब्सक्रिप्शन दोबारा पुल करना होगा।

---

## आभार

निम्नलिखित ओपन-सोर्स परियोजनाओं और समुदायों का विशेष आभार:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — कोर प्रॉक्सी इंजन और Listener कॉन्फ़िगरेशन मॉडल
- [acme.sh](https://github.com/acmesh-official/acme.sh) — SSH मेन्यू में प्रमाणपत्र आवेदन / परिनियोजन / नवीनीकरण
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener कॉन्फ़िगरेशन उदाहरण और प्रोटोकॉल संदर्भ
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — पैनल इंटरैक्शन और संचालन विचारों का संदर्भ
- [Gin](https://github.com/gin-gonic/gin) — बैकएंड HTTP फ्रेमवर्क
- [GORM](https://github.com/go-gorm/gorm) — डेटाबेस ORM
- [React](https://github.com/facebook/react) — फ़्रंटएंड आधार
- [Ant Design](https://github.com/ant-design/ant-design) — UI घटक
- [Lucide](https://github.com/lucide-icons/lucide) — आइकन (`lucide-react`, `frontend/src/icons.tsx` के माध्यम से अनुकूलित)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo रूटिंग कॉन्फ़िगरेशन और लाइट रूल विचारों का संदर्भ
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo नियम और प्रॉक्सी-ग्रुप योजनाओं का संदर्भ
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — नियम-सेट और mihomo रूटिंग टेम्पलेट संदर्भ
- [Zustand](https://github.com/pmndrs/zustand) — फ़्रंटएंड स्टेट प्रबंधन
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js और संपूर्ण ओपन-सोर्स समुदाय

योगदानकर्ता: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## लाइसेंस

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

यह परियोजना commit `226c460` के बाद से **GNU General Public License v3.0** के अंतर्गत लाइसेंस्ड है। इससे पहले के कमिट और पहले प्रकाशित संस्करण **Eclipse Public License 2.0 (EPL-2.0)** के अंतर्गत बने रहते हैं, और यह लाइसेंस अपूरणीय (irrevocable) है।

[GNU General Public License v3.0](../../LICENSE)

तृतीय-पक्ष घटक: [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)।

---

---

## अंग्रेज़ी और अन्य भाषाएँ

पूर्ण अनुवाद (18 भाषाएँ): ऊपर दी गई भाषा लिंक देखें, या [README.en.md](./README.en.md) खोलें।
