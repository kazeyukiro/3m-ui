# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · **Tiếng Việt** · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Bảng điều khiển web quản lý máy chủ Mihomo**

> **Ưu tiên bản ổn định:** cài đặt và cập nhật mặc định chỉ dùng bản Release chính thức. Bạn phải chủ động chọn bản phát hành trước (pre-release). Gói cài đặt đầy đủ và image Docker đều đi kèm phiên bản Mihomo cố định; xem [Cài đặt, nâng cấp và khôi phục](https://3m-ui.top/guide/install).

Nhẹ, tự lưu trữ (self-hosted). Quản lý Listener, người dùng, subscription và trạng thái hoạt động của [Mihomo](https://github.com/MetaCubeX/mihomo) trên Linux.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> Tài liệu: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · Trang web: [https://3m-ui.top/](https://3m-ui.top/)

---

## Tính năng

| Hạng mục | Khả năng |
|------|------|
| **Node** | Listener theo cơ chế registry giao thức: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC, v.v.; Reality, chứng chỉ tự ký TLS, cấp chứng chỉ hàng loạt một lần nhấp, **hệ số nhân lưu lượng**, kiểm tra loại trừ lẫn nhau của các trường tầng truyền tải |
| **Người dùng** | **Tạo một lần nhấp**(tên người dùng/mật khẩu/UUID ngẫu nhiên, có thể chọn gắn toàn bộ node), gắn node, hạn mức lưu lượng, **đo lường riêng theo từng node**(tính cước = thực tế × hệ số node), ngày hết hạn, giới hạn IP/lần lấy subscription, tính từ lần sử dụng đầu tiên, gia hạn/đặt lại theo chu kỳ, nhãn nhóm, gộp subscription bên ngoài, thao tác hàng loạt, Token subscription |
| **Subscription** | Tự động nhận diện theo UA: Clash/Mihomo YAML, v2ray Base64(`?target=v2ray` luôn là Base64), sing-box JSON; tùy chọn `?target=`; trang subscription HTML; link chia sẻ TUIC/HY2 kèm tham số client như SNI/`allow_insecure` |
| **Cấu hình** | Tách biệt sinh → xác thực → áp dụng; tự động quay lui `config.yaml` trước đó khi thất bại |
| **Telegram** | Cảnh báo và lệnh quản trị (cần Token + Chat ID) |
| **Đa máy** | Đăng ký panel từ xa, kiểm tra tình trạng, đồng bộ bản sao node, gộp subscription; **chọn node cục bộ theo tên để đẩy lên máy từ xa**(mặc định bị vô hiệu hóa, có thể đổi tên/cổng) |
| **Bảo mật** | JWT, đổi mật khẩu khi đăng nhập đầu tiên, xác thực hai bước TOTP, **GitHub OAuth** tùy chọn, tiền tố đường dẫn web_path, mã hóa thông tin xác thực, CORS |
| **Vận hành** | Khởi động/dừng/cập nhật core, Geo, SSL/ACME cho panel (gồm wildcard DNS-01), sao lưu, đăng ký **Cloudflare WARP** một lần nhấp (WireGuard/MASQUE YAML) |

### Ghi chú bổ sung (giới hạn người dùng / chứng chỉ / subscription)

- [Giới hạn người dùng: IP · lấy subscription](https://3m-ui.top/guide/users-limits)
- [Lưu lượng theo node và hệ số nhân](https://3m-ui.top/guide/node-traffic)
- [SSL / ACME cho panel (HTTP-01, wildcard DNS-01)](https://3m-ui.top/guide/ssl-cert)
- [Áp dụng chứng chỉ node hàng loạt](https://3m-ui.top/guide/batch-certificate)
- [Định dạng subscription và target](https://3m-ui.top/guide/subscription-formats)
- [Node đa máy / đẩy node](https://3m-ui.top/guide/cluster)
- [Định tuyến (subscription client + đầu ra phía server)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

Giao diện: **React + Ant Design + Lucide Icons**(`frontend/`), sau khi build được nhúng vào một binary Go duy nhất.

Sản phẩm phát hành: **liên kết tĩnh hoàn toàn**(`CGO_ENABLED=0` + modernc SQLite), không cần `libsqlite3` của hệ thống, tương thích cả glibc / musl.

Gói cài đặt đầy đủ / Docker: `linux/amd64` · `arm64`. Binary panel độc lập nâng cao còn hỗ trợ thêm `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`, bạn cần tự chuẩn bị kernel tương thích.

---

## Cài đặt nhanh

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Gói cài đặt đầy đủ hỗ trợ Linux amd64 / arm64, tự động cài panel, Mihomo đi kèm và dịch vụ hệ thống. Cấu hình ban đầu và mật khẩu admin do chương trình tạo thống nhất; nếu cấu hình đã tồn tại, giá trị cũ được giữ nguyên. Kiến trúc khác có thể dùng binary độc lập và kernel tự chuẩn bị, xem [hướng dẫn cài đặt chi tiết](https://3m-ui.top/guide/install).

Sau khi cài đặt xong:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

Cập nhật:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

Gỡ cài đặt:

```bash
sudo 3m-ui uninstall
```

### Cài đặt bằng prompt AI

Sao chép prompt vào trợ lý như ChatGPT / Claude / Cursor, nêu rõ hệ điều hành của bạn (ví dụ Ubuntu 22.04) và bạn đã có SSH root hay chưa. Trợ lý chỉ nên hỗ trợ chạy script chính thức, không được viết lại thành lệnh từ nguồn không rõ ràng. Prompt đầy đủ và hướng dẫn sử dụng xem tại [tài liệu cài đặt bằng prompt AI](../ai-install-prompt.md).

Không cần sao chép thủ công đoạn văn bản dài, hãy dùng `curl` để nạp trực tiếp prompt dạng văn bản thuần vào clipboard rồi dán:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

Prompt chỉ hướng dẫn sử dụng script một dòng chính thức bên dưới, mọi lượt tải đều xác thực `SHA256SUMS`:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

Kênh thử nghiệm / phát hành trước (tùy chọn, không phải mặc định) xem [tài liệu cài đặt](../installation.md#ai-提示词安装).

---

## Cài đặt bằng Docker

Trên Linux VPS, tải `docker-compose.yml` từ bản phát hành rồi chạy trong cùng thư mục:

```bash
docker compose up -d
docker compose logs 3m-ui
```

Image là `ghcr.io/kazeyukiro/3m-ui:latest`, gồm panel và phiên bản Mihomo cố định; `latest` chỉ đi theo bản ổn định. Cấu hình, khóa bí mật ngẫu nhiên và tài khoản admin được khởi tạo tự động, mật khẩu ban đầu chỉ hiển thị một lần ở lần khởi động đầu tiên. Mặc định dùng mạng host, sau khi thêm node hãy mở các cổng TCP/UDP thực tế được sử dụng. `docker-compose.bridge.yml`, cố định phiên bản, cấu hình chứng chỉ và hướng dẫn khôi phục dữ liệu bền vững xem tại [tài liệu cài đặt](https://3m-ui.top/guide/install).

---

## Build từ mã nguồn

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

Cần có binary **Mihomo** tương thích đã được cài trên máy và trỏ tới đường dẫn của nó trong cấu hình.

---

## Điểm cần lưu ý về cấu hình

Dữ liệu và cấu hình mặc định nằm ở khoảng các vị trí sau:

| Đường dẫn | Mục đích |
|------|------|
| `/etc/3m-ui/config.yaml` | Cấu hình panel |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | Chứng chỉ tự ký của node (**hãy sao lưu cùng với cơ sở dữ liệu**) |
| `/var/lib/3m-ui/mihomo/` | Thư mục làm việc của Mihomo và `config.yaml` |

Khóa bí mật JWT / thông tin xác thực hãy dùng giá trị ngẫu nhiên riêng biệt (≥ 32 byte), không dùng giá trị ví dụ trong tài liệu.

Tùy chỉnh cổng panel: `3m-ui config port <port>` hoặc chỉnh sửa cấu hình rồi khởi động lại dịch vụ.
Node mới tạo được điền sẵn cổng lắng nghe ngẫu nhiên trong khoảng 10000～60000 (lấy mẫu nhẹ và tránh các cổng node đã nạp; hỗ trợ dải đảo ngược), bạn có thể sửa thủ công hoặc nhấn «Cổng ngẫu nhiên».

---

## Sơ đồ kiến trúc

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

## Khuyến nghị sao lưu

Hãy sao lưu ít nhất:

1. `3m-ui.db`
2. `listener-certs/`
3. `config.yaml` của panel và thư mục cấu hình Mihomo

Khi chỉ cập nhật binary, chứng chỉ sẽ được hydrate từ đĩa trong khả năng có thể; nếu thư mục bị mất, chứng chỉ sẽ được tự ký lại và client cần lấy lại subscription.

---

## Lời cảm ơn

Đặc biệt cảm ơn các dự án mã nguồn mở và cộng đồng sau:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — động cơ proxy cốt lõi và mô hình cấu hình Listener
- [acme.sh](https://github.com/acmesh-official/acme.sh) — xin cấp/triển khai/gia hạn chứng chỉ trong menu SSH
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — ví dụ cấu hình Listener và tài liệu tham khảo giao thức
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — tham khảo tương tác và cách vận hành panel
- [Gin](https://github.com/gin-gonic/gin) — framework HTTP phía backend
- [GORM](https://github.com/go-gorm/gorm) — ORM cơ sở dữ liệu
- [React](https://github.com/facebook/react) — nền tảng frontend
- [Ant Design](https://github.com/ant-design/ant-design) — thành phần UI
- [Lucide](https://github.com/lucide-icons/lucide) — biểu tượng (`lucide-react`, được điều chỉnh qua `frontend/src/icons.tsx`)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — tham khảo cấu hình định tuyến Mihomo và ý tưởng quy tắc gọn nhẹ
- [echs-top/proxy](https://github.com/echs-top/proxy) — tham khảo quy tắc Mihomo và phương án nhóm chính sách
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — tham khảo bộ quy tắc và mẫu định tuyến mihomo
- [Zustand](https://github.com/pmndrs/zustand) — quản lý trạng thái frontend
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js và toàn bộ cộng đồng mã nguồn mở

Người đóng góp: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## Giấy phép

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

Dự án này, từ commit `226c460` trở đi, được cấp phép theo **GNU General Public License v3.0**. Các commit trước đó và những bản phát hành đã công bố vẫn chịu sự điều chỉnh của **Eclipse Public License 2.0 (EPL-2.0)**, và giấy phép này không thể bị thu hồi.

[GNU General Public License v3.0](../../LICENSE)

Thành phần bên thứ ba xem [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md).

---

---

## Các ngôn ngữ khác

Bản dịch đầy đủ (18 ngôn ngữ): xem các liên kết ngôn ngữ ở đầu trang, hoặc mở [README.en.md](./README.en.md).
