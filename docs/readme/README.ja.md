# 3m-ui

<p align="center">
  <img src="../../frontend/public/logo.png" alt="3m-ui logo" width="160" />
</p>

> Languages / 语言: [README](../../README.md) · [English](./README.en.md) · [简体中文](./README.zh-CN.md) · [繁體中文](./README.zh-TW.md) · **日本語** · [한국어](./README.ko.md) · [Español](./README.es.md) · [Français](./README.fr.md) · [Deutsch](./README.de.md) · [Русский](./README.ru.md) · [Português (Brasil)](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Bahasa Indonesia](./README.id.md) · [ไทย](./README.th.md) · [Türkçe](./README.tr.md) · [العربية](./README.ar.md) · [हिन्दी](./README.hi.md) · [Polski](./README.pl.md) · [Українська](./README.uk.md)

**Mihomo サーバー向け Web 管理パネル**

> **安定版優先：** 既定のインストールと更新は正式 Release のみを使用します。プレリリースは明示的に選択した場合に限られます。完全インストールパッケージおよび Docker イメージには固定バージョンの Mihomo が同梱されます。詳細は [インストール、アップグレードと復元](https://3m-ui.top/guide/install) を参照してください。

軽量・セルフホスト型。Linux 上で [Mihomo](https://github.com/MetaCubeX/mihomo) の Listener、ユーザー、購読、稼働状態を管理します。

[![Go](https://img.shields.io/github/go-mod/go-version/kazeyukiro/3m-ui?filename=backend%2Fgo.mod)](../../backend/go.mod)
> ドキュメント：[https://3m-ui.top/docs/](https://3m-ui.top/docs/) · ウェブサイト：[https://3m-ui.top/](https://3m-ui.top/)

---

## 機能

| 区分 | 内容 |
|------|------|
| **ノード** | プロトコルレジストリ駆動の Listener：VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / TUIC / AnyTLS / Snell / ShadowQUIC など；Reality、TLS 自己署名、ワンクリック一括証明書、**トラフィック倍率**、トランスポート層フィールドの排他検証 |
| **ユーザー** | **ワンクリック作成**（ランダムなユーザー名/パスワード/UUID、全ノードへのバインドも選択可）、ノードのバインド、トラフィック上限、**ノードごとの個別計量**（課金 = 実測値 × ノード倍率）、有効期限、IP/購読取得制限、初回利用からの起算、周期的な更新/リセット、グループタグ、外部購読のマージ、一括操作、購読 Token |
| **購読** | UA による自動判別 — Clash/Mihomo YAML、v2ray Base64（`?target=v2ray` は常に Base64）、sing-box JSON；任意の `?target=`；HTML 購読ページ；TUIC/HY2 の共有リンクに SNI/`allow_insecure` などのクライアントパラメータを含める |
| **設定** | 生成 → 検証 → 適用を分離；失敗時は直前の `config.yaml` にロールバック |
| **Telegram** | アラートと管理コマンド（Token + Chat ID が必要） |
| **マルチホスト** | リモートパネルの登録、ヘルスチェック、ノードミラーの同期、購読のマージ；**名前で選択したローカルノードをリモートへプッシュ**（既定は無効、名前やポートは変更可能） |
| **セキュリティ** | JWT、初回ログイン時のパスワード変更、TOTP 二段階認証、任意の **GitHub OAuth**、web_path パスプレフィックス、資格情報の暗号化、CORS |
| **運用** | コアの起動/停止/更新、Geo、パネル SSL/ACME（DNS-01 ワイルドカードを含む）、バックアップ、**Cloudflare WARP** のワンクリック登録（WireGuard/MASQUE YAML） |

### 補足説明（ユーザー制限 / 証明書 / 購読）

- [ユーザー制限：IP · 購読取得](https://3m-ui.top/guide/users-limits)
- [ノードごとのトラフィックと倍率](https://3m-ui.top/guide/node-traffic)
- [パネル SSL / ACME（HTTP-01、DNS-01 ワイルドカード）](https://3m-ui.top/guide/ssl-cert)
- [ノード証明書の一括適用](https://3m-ui.top/guide/batch-certificate)
- [購読フォーマットと target](https://3m-ui.top/guide/subscription-formats)
- [マルチホストノード / プッシュ](https://3m-ui.top/guide/cluster)
- [ルーティング（クライアント購読 + サーバー側アウトバウンド）](https://3m-ui.top/guide/routing)
- [Cloudflare WARP](https://3m-ui.top/guide/warp)

フロントエンド：**React + Ant Design + Lucide Icons**（`frontend/`）。ビルド後は単一の Go バイナリに埋め込まれます。

リリース成果物：**完全な静的リンク**（`CGO_ENABLED=0` + modernc SQLite）。システムの `libsqlite3` は不要で、glibc / musl の両方に対応します。

完全インストールパッケージ / Docker：`linux/amd64` · `arm64`。高度なスタンドアロンパネルバイナリはさらに `armv7` · `armv6` · `386` · `riscv64` · `loong64` · `ppc64le` · `s390x` に対応します（互換カーネルは各自で用意してください）。

---

## クイックインストール

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

完全インストールは Linux amd64 / arm64 に対応し、パネル、同梱の Mihomo、システムサービスを自動的にセットアップします。初期設定と管理者パスワードはプログラムが一括生成します。設定が既に存在する場合、その値は保持されます。その他のアーキテクチャではスタンドアロンバイナリと各自で用意したカーネルを使用できます。詳細は [詳細インストール手順](https://3m-ui.top/guide/install) を参照してください。

インストール完了後：

```text
面板地址   http://SERVER_IP:8080/
默认账号   admin
初始密码   安装时随机生成并显示一次，首次登录必须修改
管理命令   3m-ui
```

更新：

```bash
# 推荐（已安装后）
sudo 3m-ui update
# 指定版本: sudo 3m-ui update v1.0.6
# 从旧安装脚本迁移时，先重新执行上面的最新安装命令，配置和账号会保留。
```

アンインストール：

```bash
sudo 3m-ui uninstall
```

### AI プロンプトによるインストール

プロンプトを ChatGPT / Claude / Cursor などのアシスタントに貼り付け、使用システム（例：Ubuntu 22.04）と root SSH が利用可能かを伝えてください。アシスタントは公式スクリプトの実行のみを補助し、出所不明のコマンドへ書き換えてはなりません。完全なプロンプトと使用説明は [AI プロンプトインストールドキュメント](../ai-install-prompt.md) を参照してください。

長いテキストを手動でコピーする必要はありません。`curl` でプレーンテキストのプロンプトを直接クリップボードに読み込んで貼り付けられます：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | pbcopy        # macOS
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | xclip -sel clip # Linux X11
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/docs/ai-install-prompt.en.txt | wl-copy       # Linux Wayland
```

プロンプトは以下の公式ワンライナースクリプトのみを使用するよう誘導し、すべてのダウンロードで `SHA256SUMS` を検証します：

```bash
curl -fsSL https://raw.githubusercontent.com/kazeyukiro/3m-ui/main/scripts/install.sh | sudo sh
```

テスト / プレリリースチャンネル（任意、既定では無効）は [インストールドキュメント](../installation.md#ai-提示词安装) を参照してください。

---

## Docker インストール

Linux VPS でリリースに含まれる `docker-compose.yml` をダウンロードし、同じディレクトリで次を実行します：

```bash
docker compose up -d
docker compose logs 3m-ui
```

イメージは `ghcr.io/kazeyukiro/3m-ui:latest` で、パネルと固定バージョンの Mihomo を含みます。`latest` は安定版のみを追従します。設定、ランダムシークレット、管理者は自動的に初期化され、初期パスワードは初回起動時に一度だけ出力されます。既定は host ネットワークです。ノードを追加したら、実際に使用する TCP/UDP ポートを開放してください。`docker-compose.bridge.yml`、バージョン固定、証明書設定、永続化と復元の説明は [インストールドキュメント](https://3m-ui.top/guide/install) にあります。

---

## ソースからビルド

```bash
# 前端
cd frontend && npm install && npm run build

# 嵌入并编译后端（示例 amd64）
rm -rf backend/cmd/server/web/dist
cp -r frontend/dist backend/cmd/server/web/dist
cd backend
CGO_ENABLED=0 go build -tags sqlite_modernc -trimpath -ldflags='-s -w' -o ../3m-ui ./cmd/server
```

ローカルに互換性のある **Mihomo** バイナリがインストールされ、設定でそのパスが指定されている必要があります。

---

## 設定のポイント

既定のデータと設定の場所は概ね次のとおりです：

| パス | 用途 |
|------|------|
| `/etc/3m-ui/config.yaml` | パネル設定 |
| `/var/lib/3m-ui/3m-ui.db` | SQLite |
| `/var/lib/3m-ui/listener-certs/` | ノードの自己署名証明書（**データベースと併せてバックアップしてください**） |
| `/var/lib/3m-ui/mihomo/` | Mihomo の作業ディレクトリと `config.yaml` |

JWT / 資格情報のシークレットには個別のランダム値（32 バイト以上）を使用し、ドキュメントのサンプル値を使用しないでください。

パネルポートのカスタマイズ：`3m-ui config port <port>`、または設定を編集してサービスを再起動します。
新規ノードのリスンポートは既定で 10000〜60000 の範囲からランダムに入力されます（軽量サンプリングで読み込み済みノードのポートを回避し、降順レンジにも対応）。手動で変更するか「ランダムポート」をクリックできます。

---

## アーキテクチャ概要

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

## バックアップの推奨事項

少なくとも次をバックアップしてください：

1. `3m-ui.db`
2. `listener-certs/`
3. パネルの `config.yaml` と Mihomo 設定ディレクトリ

バイナリのみを更新した場合、証明書は可能な限りディスクから hydrate されます。ディレクトリが失われた場合は再び自己署名が行われるため、クライアントは購読を再取得する必要があります。

---

## 謝辞

特に次のオープンソースプロジェクトとコミュニティに感謝します：

- [Mihomo](https://github.com/MetaCubeX/mihomo) — コアプロキシエンジンおよび Listener 設定モデル
- [acme.sh](https://github.com/acmesh-official/acme.sh) — SSH メニューでの証明書の申請/デプロイ/更新
- [clashmeta-inbound](https://github.com/Tychristine/clashmeta-inbound/) — Listener 設定例とプロトコルリファレンス
- [3x-ui](https://github.com/MHSanaei/3x-ui) / [s-ui](https://github.com/alireza0/s-ui) — パネルの操作性と運用手法の参考
- [Gin](https://github.com/gin-gonic/gin) — バックエンド HTTP フレームワーク
- [GORM](https://github.com/go-gorm/gorm) — データベース ORM
- [React](https://github.com/facebook/react) — フロントエンド基盤
- [Ant Design](https://github.com/ant-design/ant-design) — UI コンポーネント
- [Lucide](https://github.com/lucide-icons/lucide) — アイコン（`lucide-react`、`frontend/src/icons.tsx` で調整）
- [AIsouler/MyClash](https://github.com/AIsouler/MyClash) — Mihomo ルーティング設定と軽量ルールの参考
- [echs-top/proxy](https://github.com/echs-top/proxy) — Mihomo ルールとポリシーグループ構成の参考
- [YiXuanZX/rules](https://github.com/YiXuanZX/rules) — ルールセットと mihomo ルーティングテンプレートの参考
- [Zustand](https://github.com/pmndrs/zustand) — フロントエンド状態管理
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT
- Go、Node.js およびオープンソースコミュニティ全体

貢献者：[Contributors](https://github.com/kazeyukiro/3m-ui/graphs/contributors)

---

## Star History

<a href="https://www.star-history.com/?repos=kazeyukiro%2F3m-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=kazeyukiro/3m-ui&type=date&legend=top-left" />
 </picture>
</a>

## ライセンス

[![License: GPL v3](https://www.gnu.org/graphics/gplv3-rounded-red-180x60.jpg)](../../LICENSE)

本プロジェクトは commit `226c460` 以降、**GNU General Public License v3.0** を採用しています。それより前のコミットおよび既に公開されたリリースは引き続き **Eclipse Public License 2.0（EPL-2.0）** に従い、この許諾は撤回できません。

[GNU General Public License v3.0](../../LICENSE)

サードパーティコンポーネントは [THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md) を参照してください。

---

---

## 他の言語

完全な翻訳（18 言語）は上部の言語リンクを参照するか、[README.en.md](./README.en.md) を開いてください。
