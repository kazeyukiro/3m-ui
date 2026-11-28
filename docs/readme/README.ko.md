# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · [日本語](./README.ja.md) · **한국어** · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo 서버용 웹 관리 패널**

> **안정판 우선:** 기본 설치와 업데이트는 정식 Release만 사용합니다. 프리릴리스는 직접 선택해야 합니다. 완전 설치 패키지와 Docker 이미지에는 고정된 버전의 Mihomo가 포함됩니다. 자세한 내용은 [설치, 업그레이드 및 복원](https://3m-ui.top/guide/install)을 참조하세요.

가볍고 자체 호스팅 가능하며, Linux에서 [Mihomo](https://github.com/MetaCubeX/mihomo) Listener, 사용자, 구독, 실행 상태를 관리합니다.

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> 문서: [https://3m-ui.top/docs/](https://3m-ui.top/docs/) · 웹사이트: [https://3m-ui.top/](https://3m-ui.top/)

---

## 기능

| 구분 | 내용 |
|------|------|
| **노드** | 프로토콜 레지스트리 기반 Listener: VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC 등; Reality, TLS 자체 서명, 원클릭 일괄 인증서, **트래픽 배율**, 전송 계층 필드의 상호 배타 검증 |
| **사용자** | **원클릭 생성**(무작위 사용자 이름/비밀번호/UUID, 전체 노드 바인딩 선택 가능), 노드 바인딩, 트래픽 한도, **노드별 개별 계량**(과금 = 실제 사용량 × 노드 배율), 만료, IP/구독 조회 제한, 최초 사용 시점부터 기산, 주기적 갱신/초기화, 그룹 태그, 외부 구독 병합, 일괄 작업, 구독 Token |
| **구독** | UA 자동 판별 — Clash/Mihomo YAML, v2ray Base64(`?target=v2ray` 는 항상 Base64), sing-box JSON; 선택적 `?target=`; HTML 구독 페이지; TUIC/HY2 공유 링크에 SNI/`allow_insecure` 등 클라이언트 파라미터 포함 |
| **설정** | 생성 → 검증 → 적용 분리; 실패 시 이전 `config.yaml`로 롤백 |
| **Telegram** | 알림 및 관리 명령(Token + Chat ID 필요) |
| **멀티 호스트** | 원격 패널 등록, 헬스 체크, 노드 미러 동기화, 구독 병합; **이름으로 선택한 로컬 노드를 원격으로 푸시**(기본 비활성, 이름/포트 변경 가능) |
| **보안** | JWT, 최초 로그인 시 비밀번호 변경, TOTP 2단계 인증, 선택적 **GitHub OAuth**, web_path 경로 접두사, 자격 증명 암호화, CORS |
| **운영** | 코어 시작/중지/업데이트, Geo, 패널 SSL/ACME(DNS-01 와일드카드 포함), 백업, **Cloudflare WARP** 원클릭 등록(WireGuard/MASQUE YAML) |

### 추가 설명 (사용자 제한 / 인증서 / 구독)

- [사용자 제한: IP · 구독 조회](https://3m-ui.top/guide/users-limits)
- [노드별 트래픽과 배율](https://3m-ui.top/guide/node-traffic)
- [패널 SSL / ACME(HTTP-01, DNS-01 와일드카드)](https://3m-ui.top/guide/ssl-cert)
- [노드 인증서 일괄 적용](https://3m-ui.top/guide/batch-certificate)
- [구독 형식과 target](https://3m-ui.top/guide/subscription-formats)
- [멀티 호스트 노드 / 푸시](https://3m-ui.top/guide/cluster)
- [라우팅(클라이언트 구독 + 서버 아웃바운드)](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

프론트엔드: **React + Ant Design + Lucide Icons**(`frontend/`). 빌드 후 단일 Go 바이너리에 내장됩니다.

릴리스 산출물: **완전 정적 링크**(`CGO_ENABLED=0` + modernc SQLite). 시스템 `libsqlite3`이 필요하지 않으며 glibc / musl 모두 호환됩니다.

완전 설치 패키지 / Docker: `linux/amd64` · `arm64`. 고급 독립 패널 바이너리는 추가로 `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x`를 지원하며, 호환 커널은 직접 준비해야 합니다.

---

## 빠른 설치

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

완전 설치는 Linux amd64 / arm64를 지원하며 패널, 함께 제공되는 Mihomo, 시스템 서비스를 자동으로 설치합니다. 최초 설정과 관리자 비밀번호는 프로그램이 일괄 생성합니다. 설정이 이미 있으면 기존 값이 유지됩니다. 다른 아키텍처는 독립 바이너리와 직접 준비한 커널을 사용할 수 있으며, [자세한 설치 안내](https://3m-ui.top/guide/install)를 참조하세요.

설치 완료 후:

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

업데이트:

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

제거:

```bash
sudo 3m-ui uninstall
```

### AI 프롬프트 설치

프롬프트를 ChatGPT / Claude / Cursor 등 어시스턴트에 복사하고 사용 중인 시스템(예: Ubuntu 22.04)과 root SSH 보유 여부를 알려주세요. 어시스턴트는 공식 스크립트 실행만 도와야 하며, 출처가 불분명한 명령으로 재작성해서는 안 됩니다. 전체 프롬프트와 사용 방법은 [AI 프롬프트 설치 문서](../ai-install-prompt.md)를 참조하세요.

긴 텍스트를 직접 복사할 필요 없이, `curl`로 평문 프롬프트를 클립보드에 바로 불러와 붙여넣을 수 있습니다:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

프롬프트는 아래 공식 원라인 스크립트만 사용하도록 안내하며, 모든 다운로드는 `SHA256SUMS`를 검증합니다:

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

테스트 / 프리릴리스 채널(선택 사항, 기본 아님)은 [설치 문서](../installation.md#ai-提示词安装)를 참조하세요.

---

## Docker 설치

Linux VPS에서 릴리스에 포함된 `docker-compose.yml`을 내려받아 같은 디렉터리에서 실행합니다:

```bash
docker compose up -d
docker compose logs 3m-ui
```

이미지는 `ghcr.io/kazeyukiro/3m-ui:latest`이며 패널과 고정 버전의 Mihomo를 포함합니다. `latest`는 안정판만 추종합니다. 설정, 무작위 시크릿, 관리자 계정이 자동으로 초기화되며 최초 비밀번호는 첫 시작 시 한 번만 출력됩니다. 기본은 host 네트워크이며, 노드를 추가한 뒤 실제로 사용하는 TCP/UDP 포트를 열어주세요. `docker-compose.bridge.yml`, 버전 고정, 인증서 설정, 영속화 및 복원 안내는 [설치 문서](https://3m-ui.top/guide/install)에 있습니다.

---

## 소스에서 빌드

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

로컬에 호환되는 **Mihomo** 바이너리가 설치되어 있고 설정에서 해당 경로를 지정해야 합니다.

---

## 설정 요점

기본 데이터와 설정 위치는 대략 다음과 같습니다:

| 경로 | 용도 |
|------|------|
| `/etc/3m-ui/config.yaml` | 패널 설정 |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | 노드 자체 서명 인증서(**데이터베이스와 함께 백업하세요**) |
| `/var/lib/3m-ui/mihomo/` | Mihomo 작업 디렉터리와 `config.yaml` |

JWT / 자격 증명 시크릿은 개별 무작위 값(32바이트 이상)을 사용하고, 문서의 예시 값을 사용하지 마세요.

패널 포트 변경: `3m-ui config port <port>` 또는 설정을 편집한 뒤 서비스를 재시작합니다.
새 노드의 리스닝 포트는 기본적으로 10000～60000 사이의 무작위 값이 입력됩니다(가벼운 샘플링으로 이미 로드된 노드 포트를 회피하며, 내림차순 범위도 지원). 직접 수정하거나 「랜덤 포트」를 클릭할 수 있습니다.

---

## 아키텍처 개요

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

## 백업 권장 사항

최소한 다음을 백업하세요:

1. `3m-ui.db`
2. `listener-certs/`
3. 패널 `config.yaml`과 Mihomo 설정 디렉터리

바이너리만 업데이트한 경우 인증서는 가능한 한 디스크에서 hydrate됩니다. 디렉터리가 유실되면 다시 자체 서명되므로 클라이언트는 구독을 다시 가져와야 합니다.

---

## 감사의 말

다음 오픈소스 프로젝트와 커뮤니티에 특별히 감사드립니다:

- [Mihomo](https://github.com/MetaCubeX/mihomo) — 코어 프록시 엔진 및 Listener 설정 모델
- [acme.sh](https://github.com/acmesh-official/acme.sh) — SSH 메뉴에서의 인증서 발급/배포/갱신
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 설정 예시와 프로토콜 참고 자료
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — 패널 상호작용과 운영 방식 참고
- [Gin](https://github.com/gin-gonic/gin) — 백엔드 HTTP 프레임워크
- [GORM](https://github.com/go-gorm/gorm) — 데이터베이스 ORM
- [React](https://github.com/facebook/react) — 프론트엔드 기반
- [Ant Design](https://github.com/ant-design/ant-design) — UI 컴포넌트
- [Lucide](https://github.com/lucide-icons/lucide) — 아이콘(`lucide-react`, `frontend/src/icons.tsx`에서 적용)
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo 분류 설정과 경량 규칙 아이디어 참고
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo 규칙과 정책 그룹 구성 참고
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — 규칙 세트와 mihomo 분류 템플릿 참고
- [Zustand](https://github.com/pmndrs/zustand) — 프론트엔드 상태 관리
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go, Node.js 그리고 오픈소스 커뮤니티 전체

기여자: [Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## 라이선스

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

이 프로젝트는 commit `226c460` 이후 **GNU General Public License v3.0**을 채택합니다. 그 이전의 커밋과 이미 배포된 릴리스는 계속 **Eclipse Public License 2.0(EPL-2.0)**의 적용을 받으며, 이 라이선스는 철회할 수 없습니다.

[GNU General Public License v3.0](../../LICENSE)

서드파티 구성 요소는 [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)를 참조하세요.

---

---

## 다른 언어

전체 번역(18개 언어)은 상단의 언어 링크를 참조하거나 [README.en.md](./README.en.md)를 열어보세요.
