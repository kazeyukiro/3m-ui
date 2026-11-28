package converter

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"github.com/kazeyukiro/3m-ui/backend/internal/substore/engine"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// SubStoreEnabled 返回是否使用 Sub-Store 转换核心处理非 Clash 订阅输出。
// 默认开启；设置 SUB_STORE_CONVERTER=0|false|off 可紧急回退到原生渲染。
func SubStoreEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SUB_STORE_CONVERTER")))
	return v != "0" && v != "false" && v != "off" && v != "disabled"
}

// userShareLinkBody 复用订阅节点生成逻辑，产出"订阅原文"（每行一条 share URI）。
// 这段逻辑与 GenerateUserBase64Subscription 完全一致，只是不做 base64 编码，
// 以便交给 Sub-Store 转换核心解析。
func userShareLinkBody(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is not initialized")
	}
	if gen == nil {
		return "", fmt.Errorf("URI generator is required")
	}
	if !user.IsCredentialActive(pu) {
		return "", fmt.Errorf("user is not active")
	}

	listeners, filtered, err := userBoundListeners(db, pu)
	if err != nil && !strings.Contains(err.Error(), "not bound") && !strings.Contains(err.Error(), "no exportable") {
		return "", err
	}
	if err != nil {
		listeners, filtered = nil, map[uint][]user.Credential{}
	}
	serverHost := ResolveServerAddress(config.GlobalConfig, req)

	var links []string
	var skipReasons []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		uris, err := gen(listener, host, creds)
		if err != nil || len(uris) == 0 {
			pcreds := make([]protocol.UserCred, 0, len(creds))
			for _, c := range creds {
				pcreds = append(pcreds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
			}
			if shares, err2 := protocol.ExportShareURIs(listener, host, pcreds); err2 == nil && len(shares) > 0 {
				uris = shares
				err = nil
			}
		}
		if err != nil {
			skipReasons = append(skipReasons, fmt.Sprintf("%s: %v", listener.Name, err))
			continue
		}
		if len(uris) == 0 {
			skipReasons = append(skipReasons, fmt.Sprintf("%s: empty URI export", listener.Name))
			continue
		}
		for _, u := range uris {
			u = strings.TrimSpace(u)
			if u != "" {
				links = append(links, u)
			}
		}
	}
	// 远端集群镜像节点 + 外部订阅链接，都作为订阅原文输入 Sub-Store。
	if mirrors, mErr := loadBoundRemoteMirrors(db, pu.ID); mErr == nil && len(mirrors) > 0 {
		links = appendRemoteShareURIs(mirrors, links)
	}
	if ext := strings.TrimSpace(pu.ExternalLinks); ext != "" {
		links = appendExternalShareLinks(links, ext)
	}
	if len(links) == 0 {
		if len(skipReasons) > 0 {
			return "", fmt.Errorf("no exportable share links for user (%s)", strings.Join(skipReasons, "; "))
		}
		return "", fmt.Errorf("no exportable share links for user")
	}
	return strings.Join(links, "\n"), nil
}

// appendExternalShareLinks 把外部订阅链接（可能是 URL 或 URI 列表）并入订阅原文。
// 保留外部链接的原始文本，由 Sub-Store parse 处理。
func appendExternalShareLinks(links []string, external string) []string {
	for _, line := range strings.Split(external, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			links = append(links, line)
		}
	}
	return links
}

// targetToSubStore 把 3m-ui 的 target 参数映射到 Sub-Store producer 名称。
func targetToSubStore(target string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "loon":
		return "Loon", true
	case "surge":
		return "Surge", true
	case "quanx", "quantumult", "qx":
		return "QX", true
	case "v2ray", "base64":
		return "v2ray", true
	case "uri", "raw":
		return "uri", true
	case "stash":
		return "Stash", true
	case "shadowrocket":
		return "Shadowrocket", true
	case "surfboard":
		return "Surfboard", true
	case "egern":
		return "Egern", true
	case "json":
		return "JSON", true
	default:
		return "", false
	}
}

// GenerateUserViaSubStore 使用 Sub-Store 转换核心生成订阅输出。
// 仅 Clash YAML（mihomo/clash/meta）不走此路径——保留原生生成。
func GenerateUserViaSubStore(db *gorm.DB, pu models.ProxyUser, req *http.Request, target string, gen URIGenerator) ([]byte, error) {
	producer, ok := targetToSubStore(target)
	if !ok {
		return nil, fmt.Errorf("target %q is not handled by Sub-Store", target)
	}
	raw, err := userShareLinkBody(db, pu, req, gen)
	if err != nil {
		return nil, err
	}

	var result string
	err = engine.DefaultPool().With(func(e *engine.Engine) error {
		var convErr error
		result, convErr = e.Convert(raw, producer, map[string]interface{}{
			"include-unsupported-proxy": false,
		})
		return convErr
	})
	if err != nil {
		return nil, err
	}
	// Sub-Store 的 v2ray producer 已输出 base64；uri producer 输出明文。
	// target=uri|raw 且 encrypt=0 时，路由层会按原逻辑尝试 base64 解码——
	// 但 uri 输出本就是明文，解码会失败并保留原文，行为正确。
	return []byte(result), nil
}
