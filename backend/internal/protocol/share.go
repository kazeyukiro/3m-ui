package protocol

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
)

// ShareBuilder is implemented by protocol modules that can emit client share links.
type ShareBuilder interface {
	BuildShare(in ShareInput) (Share, error)
}

// BuildShare dispatches to the protocol module when it implements ShareBuilder.
func (r Registry) BuildShare(in ShareInput) (Share, error) {
	kind := strings.ToLower(strings.TrimSpace(in.Node.Protocol))
	m, ok := r.modules[kind]
	if !ok {
		return Share{}, fmt.Errorf("unsupported protocol %q", kind)
	}
	sb, ok := m.(ShareBuilder)
	if !ok {
		return Share{}, fmt.Errorf("protocol %q does not implement share export", kind)
	}
	if !in.Node.Enabled {
		return Share{}, fmt.Errorf("disabled node cannot be shared")
	}
	return sb.BuildShare(in)
}

// BuildShares builds one Share per user credential.
func (r Registry) BuildShares(node NodeModel) ([]Share, error) {
	if len(node.Users) == 0 {
		return nil, fmt.Errorf("no users available for share export")
	}
	out := make([]Share, 0, len(node.Users))
	for _, u := range node.Users {
		s, err := r.BuildShare(ShareInput{Node: node, User: u})
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func shareHostPort(n NodeModel, host string) (string, string, error) {
	h := netutil.NormalizeHost(host)
	if h == "" {
		h = netutil.NormalizeHost(n.PublicHost)
	}
	if h == "" {
		return "", "", fmt.Errorf("public host is required for share export")
	}
	port := strings.TrimSpace(n.PublicPort)
	if port == "" {
		port = strings.TrimSpace(n.Port)
	}
	if port == "" || strings.ContainsAny(port, ",-") {
		return "", "", fmt.Errorf("share export requires a single port")
	}
	return h, port, nil
}

func shareQuery(base string, params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}

func shareName(uri, name string) string {
	if name == "" {
		return uri
	}
	return uri + "#" + url.QueryEscape(name)
}

// WithHost returns a copy with PublicHost overridden (request host resolution).
func (n NodeModel) WithHost(host string) NodeModel {
	if h := netutil.NormalizeHost(host); h != "" {
		n.PublicHost = h
	}
	return n
}
