package converter

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// client_apple.go implements native subscription exports for Loon, Surge and
// Quantumult X (QX). All three clients grew node-level parsing of the same
// share links the panel already emits for v2ray-style clients, so the export
// pipeline reuses GenerateUserBase64Subscription and re-renders each parsed
// share node into the target client's configuration grammar:
//
//   - Loon:  [Proxy] line grammar per nsloon.app/en/docs/Node
//   - Surge: [Proxy] line grammar per manual.nssurge.com
//   - QX:    [server_local] line grammar per Quantumult X sample.conf
//
// Clients that cannot speak a protocol (QX has no Hysteria2/TUIC, Loon has no
// TUIC) get a commented-out entry instead of a silently broken node.

// shareNode is a protocol-agnostic share link representation. The Names /
// Params keys follow the panel's own URI export vocabulary (see
// internal/node/uri_builders.go): insecure/allowInsecure, sni, alpn, obfs,
// obfs-password, flow, pbk, sid, type/path/host, up/down, fp, plugin, security.
type shareNode struct {
	Name     string
	Protocol string // ss, vmess, vless, trojan, hysteria2, tuic, anytls
	User     string // userinfo before ':' (tuic uuid, ss userpass mode)
	Password string // userinfo after ':' (hysteria2/trojan/vless/anytls password, ss password)
	Method   string // shadowsocks cipher
	Host     string
	Port     string
	Params   map[string]string

	// VMess links are base64 JSON instead of userinfo@host; the decoded object
	// is kept verbatim so renderers can read add/port/id/aid/scy/net/tls/sni.
	VMess *vmessLink
}

type vmessLink struct {
	Add  string `json:"add"`
	Port string `json:"port"`
	ID   string `json:"id"`
	AID  string `json:"aid"`
	Scy  string `json:"scy"`
	Net  string `json:"net"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	Host string `json:"host"`
	Path string `json:"path"`
	PS   string `json:"ps"`
	Pbk  string `json:"pbk"`
	Sid  string `json:"sid"`
}

// parseShareURI decodes one of the panel's own share links. It is intentionally
// lenient: unknown schemes return an error the caller turns into a skip comment.
func parseShareURI(uri string) (*shareNode, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, fmt.Errorf("empty uri")
	}
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return nil, fmt.Errorf("missing scheme: %s", uri)
	}
	proto := strings.ToLower(scheme)
	// Split the fragment (display name) off before the query string: the
	// fragment offsets differ between the full URI and rest, so cut it here.
	var name string
	if i := strings.Index(rest, "#"); i >= 0 {
		if n, err := url.PathUnescape(rest[i+1:]); err == nil {
			name = n
		}
		rest = rest[:i]
	}
	// Query string.
	var query string
	if i := strings.Index(rest, "?"); i >= 0 {
		query = rest[i+1:]
		rest = rest[:i]
	}
	params := map[string]string{}
	if query != "" {
		for _, kv := range strings.Split(query, "&") {
			if kv == "" {
				continue
			}
			k, v, _ := strings.Cut(kv, "=")
			if k == "" {
				continue
			}
			if dv, err := url.QueryUnescape(v); err == nil {
				v = dv
			}
			if _, dup := params[k]; !dup {
				params[lowerParamKey(proto, k)] = v
			}
		}
	}

	n := &shareNode{Params: params, Name: name}

	switch proto {
	case "hysteria2", "hy2":
		n.Protocol = "hysteria2"
	case "ss", "shadowsocks":
		n.Protocol = "ss"
	case "vmess":
		n.Protocol = "vmess"
	case "vless":
		n.Protocol = "vless"
	case "trojan":
		n.Protocol = "trojan"
	case "tuic":
		n.Protocol = "tuic"
	case "anytls":
		n.Protocol = "anytls"
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}

	if proto == "vmess" {
		obj, err := decodeVMessLink(rest)
		if err != nil {
			return nil, err
		}
		n.VMess = obj
		n.Host = obj.Add
		n.Port = strings.TrimSpace(obj.Port)
		n.Password = obj.ID
		n.Method = obj.Scy
		if obj.PS != "" && name == "" {
			n.Name = obj.PS
		}
		if n.Method == "" {
			n.Method = "auto"
		}
		// Fold vmess JSON transport fields into the shared param vocabulary.
		switch obj.Net {
		case "ws", "http":
			n.Params["type"] = obj.Net
			if obj.Path != "" {
				n.Params["path"] = obj.Path
			}
			if obj.Host != "" {
				n.Params["host"] = obj.Host
			}
		case "grpc":
			// v2rayN carries the gRPC service name in "path"; fold it or the
			// transport is invisible and every renderer falls back to tcp.
			n.Params["type"] = "grpc"
			if obj.Path != "" {
				n.Params["serviceName"] = obj.Path
			}
		}
		if obj.TLS == "tls" {
			n.Params["security"] = "tls"
			if obj.SNI != "" {
				n.Params["sni"] = obj.SNI
			}
		}
		if obj.TLS == "reality" {
			n.Params["security"] = "reality"
			if obj.SNI != "" {
				n.Params["sni"] = obj.SNI
			}
			// The panel's vmess builder puts the REALITY keys in the JSON
			// object (v2rayN field names); fold them like the query-string
			// protocols carry them or every renderer loses the public key.
			if obj.Pbk != "" {
				n.Params["pbk"] = obj.Pbk
			}
			if obj.Sid != "" {
				n.Params["sid"] = obj.Sid
			}
		}
		return n, nil
	}

	// userinfo@host:port for every non-vmess scheme.
	var authority string
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		authority = rest[i+1:]
		userinfo := rest[:i]
		if u, err := url.QueryUnescape(userinfo); err == nil {
			userinfo = u
		}
		if user, pass, hasColon := strings.Cut(userinfo, ":"); hasColon {
			n.User, n.Password = user, pass
		} else {
			// Single-secret protocols (hysteria2/trojan/vless/anytls) carry the
			// password as the whole userinfo; only tuic-style user:pass splits.
			n.Password = user
		}
		if proto == "ss" {
			// SIP002: userinfo is base64(method:password).
			if decoded, err := decodeBase64Any(userinfo); err == nil {
				if m, p, ok := strings.Cut(string(decoded), ":"); ok {
					n.Method, n.Password = m, p
				}
			} else if m, p, ok := strings.Cut(userinfo, ":"); ok {
				n.Method, n.Password = m, p
			}
		}
	} else {
		authority = rest
	}

	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		// Host without a port — keep the raw authority and let the renderer fail.
		host = authority
	}
	n.Host = strings.Trim(host, "[]")
	n.Port = strings.TrimSpace(port)
	// Hysteria2 multi-port links ("host:port1,port2-port3") mean port hopping;
	// the first port is the dial port, the remainder is a hop range.
	if proto == "hysteria2" || proto == "hy2" {
		if i := strings.Index(n.Port, ","); i >= 0 {
			hop := n.Port[i+1:]
			n.Port = n.Port[:i]
			if _, exists := n.Params["mport"]; !exists {
				n.Params["mport"] = hop
			}
		}
	}
	if n.Name == "" {
		n.Name = fmt.Sprintf("%s %s", proto, n.Host)
	}
	return n, nil
}

// lowerParamKey normalises a few well-known case variants only; realcase keys
// are preserved so renderers read exactly what the URI encoder emitted.
func lowerParamKey(proto, k string) string {
	switch strings.ToLower(k) {
	case "allowinsecure":
		return "allowInsecure"
	case "obfspassword":
		return "obfs-password"
	case "download-bandwidth":
		return "download-bandwidth"
	}
	return k
}

func decodeBase64Any(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "=")
	encodings := []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding}
	var lastErr error
	for _, enc := range encodings {
		if out, err := enc.DecodeString(s); err == nil {
			return out, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}

func decodeVMessLink(body string) (*vmessLink, error) {
	raw, err := decodeBase64Any(body)
	if err != nil {
		return nil, fmt.Errorf("vmess base64: %w", err)
	}
	var obj vmessLink
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	if obj.ID == "" || obj.Add == "" {
		return nil, fmt.Errorf("vmess link missing id/add")
	}
	return &obj, nil
}

// ---- shared helpers ----

func shareBool(params map[string]string, key string) bool {
	v, ok := params[key]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// shareInsecure reports whether the export asked the client to skip TLS
// verification (self-signed / IP-literal certificates).
func shareInsecure(params map[string]string) bool {
	return shareBool(params, "insecure") || shareBool(params, "allowInsecure") || shareBool(params, "skip-cert-verify")
}

func shareParam(params map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := params[k]; ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

// joinNodeLine renders the Loon/Surge "name = proto,server,port,…" grammar:
// the first element is the display name, the second is the protocol keyword.
func joinNodeLine(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " = " + strings.Join(parts[1:], ",")
}

// quoteValue wraps a node-line value in double quotes when the target grammar
// otherwise breaks: values containing a comma or a double quote.
func quoteValue(v string) string {
	if strings.ContainsAny(v, ",;\" ") {
		return `"` + strings.ReplaceAll(v, `"`, "") + `"`
	}
	return v
}

// mbpsValue extracts the first integer out of a bandwidth string ("30 Mbps").
func mbpsValue(v string) string {
	fields := strings.Fields(v)
	for _, f := range fields {
		if n, err := strconv.Atoi(f); err == nil && n > 0 {
			return strconv.Itoa(n)
		}
	}
	return ""
}

// commentLine renders a skipped node as a comment carrying the reason.
func commentLine(name, reason string) string {
	return fmt.Sprintf("# %s: %s", name, reason)
}

// ---- Documented value sets (strict alignment) ---------------------------
//
// Every closed set below is transcribed verbatim from the official manuals.
// A share link carrying a value outside one of these sets is degraded to a
// comment line instead of being rendered with an undocumented parameter.
//
//   Surge: manual.nssurge.com/policies/{vless,vmess,trojan,shadowsocks,
//          hysteria2,tuic,anytls,tls}.html
//   Loon:  nsloon.app/docs/Node (zh) / nsloon.app/en/docs/Node (en)

// surgeVLESSFlows - policies/vless.html: "Optional, `xtls-rprx-vision` or
// `xtls-rprx-vision-udp443`".
var surgeVLESSFlows = map[string]bool{
	"xtls-rprx-vision":        true,
	"xtls-rprx-vision-udp443": true,
}

// surgeGRPCModes - policies/vless.html: "Optional, `gun` or `multi`,
// default: `gun`".
var surgeGRPCModes = map[string]bool{"gun": true, "multi": true}

// surgeVMessMethods - policies/vmess.html: "Optional, `aes-128-gcm` or
// `chacha20-ietf-poly1305`, default: `aes-128-gcm`". The share-link value
// `auto` (the panel's default) has NO Surge counterpart.
var surgeVMessMethods = map[string]bool{
	"aes-128-gcm":            true,
	"chacha20-ietf-poly1305": true,
}

// surgeSSMethods - policies/shadowsocks.html "Supported values" (verbatim).
var surgeSSMethods = map[string]bool{
	"2022-blake3-aes-128-gcm": true, "2022-blake3-aes-256-gcm": true,
	"aes-128-gcm": true, "aes-192-gcm": true, "aes-256-gcm": true,
	"chacha20-ietf-poly1305": true, "xchacha20-ietf-poly1305": true,
	"rc4": true, "rc4-md5": true,
	"aes-128-cfb": true, "aes-192-cfb": true, "aes-256-cfb": true,
	"aes-128-ctr": true, "aes-192-ctr": true, "aes-256-ctr": true,
	"salsa20": true, "chacha20": true, "chacha20-ietf": true, "none": true,
}

// surgeObfsModes - policies/shadowsocks.html: "Optional, `http` or `tls`".
var surgeObfsModes = map[string]bool{"http": true, "tls": true}

// surgeHy2Obfs - policies/hysteria2.html documents salamander-password and
// gecko-password only ("mutually exclusive").
var surgeHy2Obfs = map[string]bool{"salamander": true, "gecko": true}

// loonVMessCiphers - docs/Node VMess: "加密方式支持 `none`、`auto`、
// `aes-128-cfb`、`aes-128-gcm`、`chacha20-ietf-poly1305`".
var loonVMessCiphers = map[string]bool{
	"none": true, "auto": true, "aes-128-cfb": true,
	"aes-128-gcm": true, "chacha20-ietf-poly1305": true,
}

// loonVLESSFlows - docs/Node VLESS: "`flow` 当前支持 `xtls-rprx-vision`".
var loonVLESSFlows = map[string]bool{"xtls-rprx-vision": true}

// loonObfsNames - docs/Node Shadowsocks: "`obfs-name` Simple Obfs 类型：
// `none`、`http`、`tls`".
var loonObfsNames = map[string]bool{"none": true, "http": true, "tls": true}

// loonHy2Obfs - docs/Node Hysteria 2 documents salamander-password only;
// there is no gecko-password parameter on that page.
var loonHy2Obfs = map[string]bool{"salamander": true}

// surgeVLESSXHTTPModes - policies/vless.html: "xhttp-mode: Optional,
// `auto`, `packet-up`, `stream-up` or `stream-one`, default: `auto`".
// The share-link `mode` field maps to grpc-mode for type=grpc and to
// xhttp-mode for type=xhttp; both sets are checked by surgeVLESSUnsupported.
var surgeVLESSXHTTPModes = map[string]bool{
	"auto": true, "packet-up": true, "stream-up": true, "stream-one": true,
}

// surgeVLESSEncPrefix - policies/vless.html: the `encryption` value is the
// client string printed by `xray vlessenc`, shaped
// `mlkem768x25519plus.<mode>.<handshake>[.<padding>].<key>...`. `none` (or an
// absent value) is the documented default. Any other value is outside the
// documented grammar and degrades instead of being emitted as-is.
const surgeVLESSEncPrefix = "mlkem768x25519plus."

// loonSSMethods - nsloon.app/docs/Node Shadowsocks "支持的加密方式" (verbatim).
// Unlike Surge, Loon's list does NOT include `none`.
var loonSSMethods = map[string]bool{
	"2022-blake3-aes-128-gcm": true, "2022-blake3-aes-256-gcm": true,
	"aes-128-gcm": true, "aes-192-gcm": true, "aes-256-gcm": true,
	"chacha20-ietf-poly1305": true, "xchacha20-ietf-poly1305": true,
	"rc4": true, "rc4-md5": true,
	"aes-128-cfb": true, "aes-192-cfb": true, "aes-256-cfb": true,
	"aes-128-ctr": true, "aes-192-ctr": true, "aes-256-ctr": true,
	"bf-cfb":           true,
	"camellia-128-cfb": true, "camellia-192-cfb": true, "camellia-256-cfb": true,
	"cast5-cfb": true, "des-cfb": true, "idea-cfb": true, "rc2-cfb": true,
	"seed-cfb": true, "salsa20": true, "chacha20": true, "chacha20-ietf": true,
}

func inSet(set map[string]bool, v string) bool {
	return set[strings.ToLower(strings.TrimSpace(v))]
}

// surgeShortIDOK - policies/vless.html: "reality-short-id ... up to 16
// hexadecimal digits". An empty value is valid (the parameter is optional).
func surgeShortIDOK(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	if len(v) > 16 {
		return false
	}
	for _, c := range v {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// wsPathOK - policies/vmess.html and policies/trojan.html: "ws-path ...
// It must start with `/`". VLESS inherits it ("See VMess for ws-path").
func wsPathOK(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.HasPrefix(v, "/")
}

// ssPluginObfs returns the `obfs=` value of a SIP003 plugin string.
func ssPluginObfs(plugin string) string {
	for _, seg := range strings.Split(plugin, ";") {
		seg = strings.TrimSpace(seg)
		k, v, _ := strings.Cut(seg, "=")
		if strings.EqualFold(k, "obfs") {
			return v
		}
	}
	return ""
}

// docQuote wraps a Loon positional credential the way every declaration
// form and example in docs/Node does: `Shadowsocks,server,port,cipher,
// "password"` / `VMess,server,port,cipher,"UUID"` / `VLESS,server,port,
// "UUID"` / `Trojan,server,port,"password"`.
func docQuote(v string) string {
	return `"` + strings.ReplaceAll(v, `"`, "") + `"`
}

// loonSkipMsg reports nodes Loon cannot express per docs/Node. The three
// empirical behaviours (AnyTLS REALITY downgrade, single-port server-ports,
// Trojan REALITY over-tls) are deliberately kept - see renderLoonNode.
func loonSkipMsg(n *shareNode) (string, bool) {
	switch n.Protocol {
	case "tuic":
		// docs/Node lists no TUIC policy at all.
		return "Loon 暂不支持 TUIC 协议", true
	case "hysteria2":
		if obfs := shareParam(n.Params, "obfs"); obfs == "gecko" {
			return "Loon 暂不支持 Hysteria2 Gecko 混淆（文档仅 salamander）", true
		} else if obfs != "" && !inSet(loonHy2Obfs, obfs) {
			return "Loon Hysteria2 混淆方式不在文档支持列表", true
		}
	case "vmess":
		if !inSet(loonVMessCiphers, n.Method) {
			return "Loon VMess 加密方式不在文档支持列表", true
		}
	case "vless":
		if f := shareParam(n.Params, "flow"); f != "" && !inSet(loonVLESSFlows, f) {
			return "Loon VLESS flow 仅支持 xtls-rprx-vision", true
		}
	case "ss":
		if !inSet(loonSSMethods, n.Method) {
			return "Loon SS 加密方式不在文档支持列表", true
		}
		if o := ssPluginObfs(n.Params["plugin"]); o != "" && !inSet(loonObfsNames, o) {
			return "Loon SS obfs-name 仅支持 none / http / tls", true
		}
	case "anytls":
		if t := n.Params["type"]; t != "" && t != "tcp" && t != "none" {
			// docs/Node AnyTLS documents no transport= argument.
			return "Loon AnyTLS 仅支持 TCP", true
		}
	}
	switch n.Protocol {
	case "vmess", "vless", "trojan":
		if !loonTransportOK(n.Params["type"]) {
			return "Loon 暂不支持该传输（VMess/VLESS: tcp/ws/http；Trojan: tcp/ws）", true
		}
	}
	return "", false
}

// ---- Loon ----

// loonTransportOK reports whether Loon's transport= vocabulary covers the
// node's transport. nsloon.app/en/docs/Node: VMess/VLESS support tcp, ws and
// http ("host and path apply to WebSocket or HTTP transports"); Trojan
// supports tcp and ws. grpc / xhttp / quic / kcp have no Loon line at all and
// must degrade to a comment instead of a silently wrong tcp node.
func loonTransportOK(typ string) bool {
	switch typ {
	case "", "tcp", "none", "ws", "http":
		return true
	}
	return false
}

// renderLoonNode renders one share node in Loon [Proxy] grammar.
// Declaration forms from docs/Node put every positional credential in
// double quotes and place `transport=` immediately after it:
//
//	Shadowsocks,server,port,cipher,"password",…
//	VMess,server,port,cipher,"UUID",transport=…,alterId=0,path=…,host=…
//	VLESS,server,port,"UUID",transport=…,flow=…,public-key="…",…
//	Trojan,server,port,"password",transport=…,path=…,host=…,alpn=…
//	Hysteria2,server,port,"password",…
//	AnyTLS,server,port,"password",…
func renderLoonNode(n *shareNode) string {
	// Never emit a line the documented grammar cannot express.
	if _, unsupported := loonSkipMsg(n); unsupported {
		return ""
	}
	var b strings.Builder
	tag := quoteValue(n.Name)
	switch n.Protocol {
	case "hysteria2":
		parts := []string{tag, "Hysteria2", n.Host, n.Port, docQuote(n.Password)}
		if sni := shareParam(n.Params, "sni", "peer"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+quoteValue(alpn))
		}
		// docs/Node Hysteria 2 documents salamander-password only.
		if shareParam(n.Params, "obfs") == "salamander" {
			if pw := shareParam(n.Params, "obfs-password"); pw != "" {
				parts = append(parts, "salamander-password="+quoteValue(pw))
			}
		}
		// Empirical (kept): Loon's Hysteria 2 plugin only dials when
		// server-ports is present, so a single-port node carries it too.
		ports := hopFormatLoon(shareParam(n.Params, "mport"))
		if ports == "" {
			ports = n.Port
		}
		parts = append(parts, `server-ports="`+strings.ReplaceAll(ports, `"`, "")+`"`)
		if bw := mbpsValue(shareParam(n.Params, "down", "up")); bw != "" {
			parts = append(parts, "download-bandwidth="+bw)
		}
		parts = append(parts, "udp=true", "block-quic=false")
		b.WriteString(joinNodeLine(parts))
	case "ss":
		if n.Method == "" {
			return ""
		}
		parts := []string{tag, "Shadowsocks", n.Host, n.Port, n.Method, docQuote(n.Password)}
		parts = append(parts, loonSSPlugin(n.Params["plugin"])...)
		parts = append(parts, "udp=true", "block-quic=false")
		b.WriteString(joinNodeLine(parts))
	case "vmess":
		parts := []string{tag, "VMess", n.Host, n.Port, n.Method, docQuote(n.Password)}
		transport := "tcp"
		path, host := "", ""
		switch n.Params["type"] {
		case "ws", "http":
			transport = n.Params["type"]
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		}
		parts = append(parts, "transport="+transport, "alterId=0")
		if path != "" {
			parts = append(parts, "path="+quoteValue(path))
		}
		if host != "" {
			parts = append(parts, "host="+host)
		}
		// docs/Node defines `flow` under VLESS only and shows no `alpn`
		// parameter on VMess/VLESS lines - pass "" for both.
		b.WriteString(loonTLSTail(parts, n.Params, "", ""))
	case "vless":
		parts := []string{tag, "VLESS", n.Host, n.Port, docQuote(n.Password)}
		transport := "tcp"
		path, host := "", ""
		switch n.Params["type"] {
		case "ws", "http":
			transport = n.Params["type"]
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		}
		parts = append(parts, "transport="+transport)
		if path != "" {
			parts = append(parts, "path="+quoteValue(path))
		}
		if host != "" {
			parts = append(parts, "host="+host)
		}
		b.WriteString(loonTLSTail(parts, n.Params, shareParam(n.Params, "flow"), ""))
	case "trojan":
		parts := []string{tag, "Trojan", n.Host, n.Port, docQuote(n.Password)}
		transport := "tcp"
		path, host := "", ""
		switch n.Params["type"] {
		case "ws", "http":
			// docs/Node: "兼容配置中的 `transport=http` 会按 WebSocket 处理".
			transport = "ws"
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		}
		parts = append(parts, "transport="+transport)
		if path != "" {
			parts = append(parts, "path="+quoteValue(path))
		}
		if host != "" {
			parts = append(parts, "host="+host)
		}
		// docs/Node Trojan examples carry `alpn` (WS: alpn="h2,http/1.1";
		// REALITY: alpn=h2). VMess/VLESS examples do not.
		b.WriteString(loonTLSTail(parts, n.Params, "", shareParam(n.Params, "alpn")))
	case "anytls":
		// Empirical (kept): docs/Node documents public-key/short-id for
		// AnyTLS REALITY, but a real-device check found Loon reliably
		// parses only the password-only form, so REALITY args are dropped.
		parts := []string{tag, "AnyTLS", n.Host, n.Port, docQuote(n.Password)}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		parts = append(parts, "udp=true", "block-quic=false")
		b.WriteString(joinNodeLine(parts))
	default:
		return ""
	}
	return b.String()
}

// loonTLSTail appends the shared Loon TLS / REALITY arguments in the order
// used by the official docs/Node examples: flow, public-key, short-id,
// over-tls, sni, alpn, skip-cert-verify, udp, block-quic.
//
// `flow` is VLESS-only (docs/Node VLESS: "`flow` 当前支持
// `xtls-rprx-vision`"); `alpn` appears only in the Trojan and Hysteria 2
// examples. Pass "" for either when the protocol does not document it, so
// no undocumented parameter can reach a policy line. `public-key` is a
// REALITY parameter (docs/Node: "public-key 和 short-id 用于 Reality") and
// is never emitted for a plain TLS node.
func loonTLSTail(parts []string, params map[string]string, flow, alpn string) string {
	security := strings.ToLower(strings.TrimSpace(params["security"]))
	if flow != "" {
		parts = append(parts, "flow="+flow)
	}
	if security == "reality" {
		if pbk := shareParam(params, "pbk"); pbk != "" {
			parts = append(parts, "public-key="+docQuote(pbk))
		}
		if sid := shareParam(params, "sid"); sid != "" {
			parts = append(parts, "short-id="+sid)
		}
	}
	if security == "tls" || security == "reality" {
		parts = append(parts, "over-tls=true")
	}
	if sni := shareParam(params, "sni"); sni != "" {
		parts = append(parts, "sni="+sni)
	}
	if alpn != "" {
		parts = append(parts, "alpn="+quoteValue(alpn))
	}
	if shareInsecure(params) {
		parts = append(parts, "skip-cert-verify=true")
	}
	parts = append(parts, "udp=true", "block-quic=false")
	return joinNodeLine(parts)
}

// loonSSPlugin maps a SIP003 plugin string (obfs-local;obfs=http;obfs-host=…)
// onto Loon's obfs-name/obfs-host/obfs-uri arguments.
func loonSSPlugin(plugin string) []string {
	if plugin == "" {
		return nil
	}
	out := []string{}
	for _, seg := range strings.Split(plugin, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.EqualFold(seg, "obfs-local") {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		if !ok {
			k, v = seg, ""
		}
		switch strings.ToLower(k) {
		case "obfs":
			if v != "" {
				out = append(out, "obfs-name="+v)
			}
		case "obfs-host":
			if v != "" {
				out = append(out, "obfs-host="+quoteValue(v))
			}
		case "obfs-uri":
			if v != "" {
				out = append(out, "obfs-uri="+quoteValue(v))
			}
		}
	}
	return out
}

// hopFormatSemicolon converts a comma hop list ("443,5000-6000") to the
// semicolon grammar Loon/Surge expect.
func hopFormatSemicolon(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.Contains(v, ";") {
		return strings.ReplaceAll(v, ",", ";")
	}
	return v
}

// hopFormatLoon renders a share-URI port-hop spec (comma-separated entries,
// dash ranges like "5000-6000") the way Loon's server-ports documents it:
// comma-separated entries with colon ranges ("5000:6000"), the value quoted
// by the caller when it contains commas (Loon separates node args by comma).
func hopFormatLoon(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	segs := strings.Split(v, ",")
	for i, s := range segs {
		s = strings.TrimSpace(s)
		if lo, hi, ok := strings.Cut(s, "-"); ok && lo != "" && hi != "" {
			segs[i] = lo + ":" + hi
		} else {
			segs[i] = s
		}
	}
	return strings.Join(segs, ",")
}

// ---- Surge ----

// surgeTransportOK reports whether Surge's VMess / Trojan transport
// vocabulary covers the node's transport. manual.nssurge.com policies/vmess
// and policies/trojan document ws (with ws-path / ws-headers) as the only
// optional transport layer — there is no grpc / xhttp / http option, so those
// links must degrade to a comment instead of a silently wrong plain-TLS node.
// VLESS is deliberately NOT covered here: policies/vless.html (Surge iOS
// 6.0.0+ / Mac 6.12.0+) adds gRPC and XHTTP alongside RAW and WebSocket,
// handled by surgeVLESSUnsupported and the vless renderer branch.
func surgeTransportOK(typ string) bool {
	switch typ {
	case "", "tcp", "none", "ws":
		return true
	}
	return false
}

// renderSurgeNode renders one share node in Surge [Proxy] grammar.
// Syntax, parameters and closed value sets come from
// manual.nssurge.com/policies/{vless,vmess,trojan,shadowsocks,hysteria2,
// tuic,anytls,tls}.html. Anything surgeSkipMsg rejects is never rendered,
// so an undocumented value can never reach a policy line.
func renderSurgeNode(n *shareNode) string {
	if _, unsupported := surgeSkipMsg(n); unsupported {
		return ""
	}
	var b strings.Builder
	tag := quoteValue(n.Name)
	switch n.Protocol {
	case "hysteria2":
		// policies/hysteria2.html: password, download-bandwidth,
		// port-hopping, port-hopping-interval, salamander-password,
		// gecko-password; shared TLS params (sni, skip-cert-verify, alpn).
		parts := []string{tag, "hysteria2", n.Host, n.Port, "password=" + quoteValue(n.Password)}
		if sni := shareParam(n.Params, "sni", "peer"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+quoteValue(alpn))
		}
		// The two obfuscation modes are mutually exclusive; surgeSkipMsg
		// already rejected any other value.
		switch shareParam(n.Params, "obfs") {
		case "salamander":
			if pw := shareParam(n.Params, "obfs-password"); pw != "" {
				parts = append(parts, "salamander-password="+quoteValue(pw))
			}
		case "gecko":
			if pw := shareParam(n.Params, "obfs-password"); pw != "" {
				parts = append(parts, "gecko-password="+quoteValue(pw))
			}
		}
		if hop := shareParam(n.Params, "mport"); hop != "" {
			// "port-hopping: separated by semicolons (e.g. 1234;5000-6000)".
			parts = append(parts, "port-hopping="+quoteValue(hopFormatSemicolon(hop)))
		}
		if bw := mbpsValue(shareParam(n.Params, "down", "up")); bw != "" {
			parts = append(parts, "download-bandwidth="+bw)
		}
		b.WriteString(joinNodeLine(parts))
	case "ss":
		if n.Method == "" {
			return ""
		}
		parts := []string{tag, "ss", n.Host, n.Port, "encrypt-method=" + strings.ToLower(n.Method), "password=" + quoteValue(n.Password)}
		parts = append(parts, surgeSSPlugin(n.Params["plugin"])...)
		parts = append(parts, "udp-relay=true")
		b.WriteString(joinNodeLine(parts))
	case "vmess":
		if !surgeTransportOK(n.Params["type"]) {
			return ""
		}
		// policies/vmess.html: `username=<UUID>[, parameter=value, ...]`.
		// encrypt-method is optional with exactly two documented values and
		// defaults to aes-128-gcm. vmess-aead is an INDEPENDENT boolean that
		// "must match the server's setting" - the manual gives no rule to
		// derive it from the cipher, so it is never emitted.
		params := []string{"username=" + n.Password}
		if m := strings.ToLower(strings.TrimSpace(n.Method)); surgeVMessMethods[m] {
			params = append(params, "encrypt-method="+m)
		}
		if n.Params["type"] == "ws" {
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+quoteValue(p))
			}
			if h := shareParam(n.Params, "host"); h != "" {
				// "ws-headers: pipe-separated Header:Value pairs".
				params = append(params, "ws-headers="+quoteValue("Host:"+h))
			}
		}
		if n.Params["security"] == "tls" {
			params = append(params, "tls=true")
			if sni := shareParam(n.Params, "sni"); sni != "" {
				params = append(params, "sni="+sni)
			}
			if alpn := shareParam(n.Params, "alpn"); alpn != "" {
				params = append(params, "alpn="+quoteValue(alpn))
			}
			if shareInsecure(n.Params) {
				params = append(params, "skip-cert-verify=true")
			}
		}
		parts := append([]string{tag, "vmess", n.Host, n.Port}, params...)
		b.WriteString(joinNodeLine(parts))
	case "vless":
		// policies/vless.html - syntax, combination matrix and the
		// share-link mapping table. All closed sets are checked by
		// surgeVLESSUnsupported via surgeSkipMsg before we get here.
		params := []string{"username=" + quoteValue(n.Password)}
		security := strings.ToLower(strings.TrimSpace(n.Params["security"]))
		// The official share-link mapping sets reality-public-key whenever
		// the link carries security=reality/pbk; the public key itself is
		// what enables REALITY ("implies TLS"), so tls=true is not emitted.
		reality := security == "reality" || strings.TrimSpace(n.Params["pbk"]) != ""
		switch strings.ToLower(strings.TrimSpace(n.Params["type"])) {
		case "", "tcp", "raw", "none":
			// RAW (TCP): no transport keyword is emitted.
		case "ws":
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+quoteValue(p))
			}
			if h := shareParam(n.Params, "host"); h != "" {
				params = append(params, "ws-headers="+quoteValue("Host:"+h))
			}
		case "grpc":
			params = append(params, "grpc=true")
			if sn := shareParam(n.Params, "serviceName"); sn != "" {
				params = append(params, "grpc-service-name="+quoteValue(sn))
			}
			if mode := strings.ToLower(shareParam(n.Params, "mode")); mode == "multi" {
				params = append(params, "grpc-mode=multi")
			}
			if auth := shareParam(n.Params, "authority"); auth != "" {
				params = append(params, "grpc-authority="+quoteValue(auth))
			}
		case "xhttp", "splithttp":
			// policies/vless.html share-link table: type=xhttp (or splithttp),
			// path, host, mode -> xhttp=true, xhttp-path, xhttp-host, xhttp-mode.
			// The mode value was validated against the documented closed set by
			// surgeVLESSUnsupported before we get here.
			params = append(params, "xhttp=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "xhttp-path="+quoteValue(p))
			}
			if h := shareParam(n.Params, "host"); h != "" {
				params = append(params, "xhttp-host="+quoteValue(h))
			}
			if mode := strings.ToLower(shareParam(n.Params, "mode")); mode != "" {
				params = append(params, "xhttp-mode="+mode)
			}
		default:
			return ""
		}
		if reality {
			params = append(params, "reality-public-key="+quoteValue(shareParam(n.Params, "pbk")))
			if sid := shareParam(n.Params, "sid"); sid != "" {
				params = append(params, "reality-short-id="+quoteValue(sid))
			}
		} else if security == "tls" {
			params = append(params, "tls=true")
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			params = append(params, "sni="+sni)
		}
		if flow := shareParam(n.Params, "flow"); flow != "" {
			params = append(params, "flow="+flow)
		}
		// VLESS Encryption (policies/vless.html `encryption`; share-link table
		// `encryption` -> `encryption`). `none` is the documented default and
		// carries no parameter of its own.
		if enc := shareParam(n.Params, "encryption"); enc != "" && !strings.EqualFold(enc, "none") {
			params = append(params, "encryption="+quoteValue(enc))
		}
		// "alpn" is in the VLESS share-link mapping table; tls.html calls it
		// a shared TLS parameter and says to quote multi-entry values.
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			params = append(params, "alpn="+quoteValue(alpn))
		}
		// VLESS page: with REALITY, "skip-cert-verify and certificate
		// pinning do not apply".
		if !reality && shareInsecure(n.Params) {
			params = append(params, "skip-cert-verify=true")
		}
		parts := append([]string{tag, "vless", n.Host, n.Port}, params...)
		b.WriteString(joinNodeLine(parts))
	case "trojan":
		if !surgeTransportOK(n.Params["type"]) {
			return ""
		}
		// policies/trojan.html: `password=<password>` is required; ws /
		// ws-path / ws-headers are the only protocol-specific parameters.
		params := []string{"password=" + quoteValue(n.Password)}
		if n.Params["type"] == "ws" {
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+quoteValue(p))
			}
			if h := shareParam(n.Params, "host"); h != "" {
				params = append(params, "ws-headers="+quoteValue("Host:"+h))
			}
		}
		// Trojan always connects over TLS, so the shared TLS parameters on
		// policies/tls.html apply: sni, alpn, skip-cert-verify.
		if sni := shareParam(n.Params, "sni"); sni != "" {
			params = append(params, "sni="+sni)
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			params = append(params, "alpn="+quoteValue(alpn))
		}
		if shareInsecure(n.Params) {
			params = append(params, "skip-cert-verify=true")
		}
		parts := append([]string{tag, "trojan", n.Host, n.Port}, params...)
		b.WriteString(joinNodeLine(parts))
	case "anytls":
		// policies/anytls.html: `password` (required), `reuse` (optional).
		// AnyTLS always uses TLS, so all shared TLS parameters apply.
		parts := []string{tag, "anytls", n.Host, n.Port, "password=" + quoteValue(n.Password)}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+quoteValue(alpn))
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		b.WriteString(joinNodeLine(parts))
	case "tuic":
		// policies/tuic.html: `tuic` (v4, token) and `tuic-v5` (uuid +
		// password). alpn defaults to h3; port-hopping is shared.
		kw := "tuic"
		parts := []string{tag, kw, n.Host, n.Port}
		if n.User != "" {
			parts[1] = "tuic-v5"
			parts = append(parts, "uuid="+n.User, "password="+quoteValue(n.Password))
		} else if n.Password != "" {
			parts = append(parts, "token="+quoteValue(n.Password))
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+quoteValue(alpn))
		} else {
			parts = append(parts, "alpn=h3")
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		if hop := shareParam(n.Params, "mport"); hop != "" {
			parts = append(parts, "port-hopping="+quoteValue(hopFormatSemicolon(hop)))
		}
		b.WriteString(joinNodeLine(parts))
	default:
		return ""
	}
	return b.String()
}

// surgeSSPlugin maps a SIP003 plugin string onto the parameters documented
// on policies/shadowsocks.html: obfs (http|tls), obfs-host, and obfs-uri
// ("Only meaningful with `obfs=http`").
func surgeSSPlugin(plugin string) []string {
	out := []string{}
	if plugin == "" {
		return out
	}
	obfs, host, uri := "", "", ""
	for _, seg := range strings.Split(plugin, ";") {
		seg = strings.TrimSpace(seg)
		if strings.EqualFold(seg, "obfs-local") {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		switch strings.ToLower(k) {
		case "obfs":
			obfs = v
		case "obfs-host":
			host = v
		case "obfs-uri":
			uri = v
		}
	}
	if obfs != "" && obfs != "none" {
		out = append(out, "obfs="+obfs)
	}
	if host != "" {
		out = append(out, "obfs-host="+quoteValue(host))
	}
	// "obfs-uri: The URI used in the obfuscation request. Only meaningful
	// with obfs=http." Never emit it for obfs=tls or when obfs is absent.
	if uri != "" && strings.EqualFold(obfs, "http") {
		out = append(out, "obfs-uri="+quoteValue(uri))
	}
	return out
}

// ---- Quantumult X ----

// renderQuanXNode renders one share node in QX [server_local] grammar
// (sample.conf v1.5.5: keyword=host:port, key=value, …, tag=name).
func renderQuanXNode(n *shareNode) string {
	head := fmt.Sprintf("%s=%s:%s", quanxProtoKeyword(n.Protocol), n.Host, n.Port)
	params := []string{}
	switch n.Protocol {
	case "ss":
		if n.Method == "" {
			return ""
		}
		plugin := n.Params["plugin"]
		if plugin != "" {
			name := strings.TrimSpace(strings.SplitN(plugin, ";", 2)[0])
			if !strings.EqualFold(name, "obfs-local") {
				// kcptun / shadow-tls / restls / jls wrappers have no QX obfs
				// counterpart (sample.conf documents obfs=http|tls|ws|wss|
				// over-tls and the SS plugin forms only).
				return ""
			}
			if sec := n.Params["security"]; sec == "tls" || sec == "reality" {
				// QX has a single obfs field; simple-obfs and TLS/Reality
				// cannot travel on the same line.
				return ""
			}
		}
		params = append(params, "method="+n.Method, "password="+quoteValue(n.Password))
		params = append(params, quanxSSPlugin(plugin)...)
	case "vmess":
		method, ok := quanxVMessMethod(n.Method)
		if !ok {
			return ""
		}
		params = append(params, "method="+method, "password="+quoteValue(n.Password))
	case "vless":
		params = append(params, "method=none", "password="+quoteValue(n.Password))
		if flow := shareParam(n.Params, "flow"); flow != "" {
			params = append(params, "vless-flow="+flow)
		}
	case "trojan":
		params = append(params, "password="+quoteValue(n.Password))
	case "anytls":
		if sec := n.Params["security"]; sec != "tls" && sec != "reality" {
			// anytls is TLS by protocol definition; the panel's anytls export
			// carries sni/insecure but no security keyword (unlike vless/
			// trojan), and QX spells AnyTLS TLS as over-tls=true.
			n.Params["security"] = "tls"
		}
		params = append(params, "password="+quoteValue(n.Password))
	default:
		return ""
	}
	tr, ok := quanxTransport(n)
	if !ok {
		return ""
	}
	params = append(params, tr...)
	params = append(params, "udp-relay=true", "tag="+quoteValue(n.Name))
	return head + ", " + strings.Join(params, ", ")
}

// quanxProtoKeyword maps the panel protocol name to QX's [server_local]
// keyword: sample.conf writes shadowsocks=… — there is no ss= form.
func quanxProtoKeyword(proto string) string {
	if proto == "ss" {
		return "shadowsocks"
	}
	return proto
}

// quanxObfsTLS reports whether the protocol family spells TLS the ss/vmess/
// vless way — obfs=over-tls with the SNI in obfs-host — instead of the
// http/socks5/trojan/anytls way, over-tls=true with tls-host. sample.conf
// demonstrates both styles (ss-tls-01 vs trojan-tls-03).
func quanxObfsTLS(proto string) bool {
	switch proto {
	case "ss", "vmess", "vless":
		return true
	}
	return false
}

// quanxVMessMethod maps a VMess cipher onto the method values sample.conf
// documents for QX: none, aes-128-gcm or chacha20-poly1305 (aead defaults to
// true). zero/none/auto — "auto" is the panel's own default — carry no body
// cipher on the wire and render as method=none; any other cipher has no QX
// counterpart and must degrade to a comment.
func quanxVMessMethod(method string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "none", "zero", "auto":
		return "none", true
	case "aes-128-gcm", "chacha20-poly1305":
		return strings.ToLower(strings.TrimSpace(method)), true
	}
	return "", false
}

// quanxReality renders the REALITY keys sample.conf documents for every
// TLS-capable protocol (ss-tls-reality-uot-01, vmess-wss-reality-01,
// vless-tls-reality-vision-01, trojan-tls-reality-01, anytls-reality-tls-01).
func quanxReality(params map[string]string) []string {
	out := []string{}
	if pbk := shareParam(params, "pbk"); pbk != "" {
		out = append(out, "reality-base64-pubkey="+quoteValue(pbk))
	}
	if sid := shareParam(params, "sid"); sid != "" {
		out = append(out, "reality-hex-shortid="+quoteValue(sid))
	}
	return out
}

// quanxAlpnHex encodes an ALPN list the way sample.conf documents the
// tls-alpn parameter: tls-alpn=02683208687474702f312e31 decodes to h2 plus
// http/1.1, i.e. each entry as one length byte followed by its ASCII bytes.
func quanxAlpnHex(alpn string) string {
	var b []byte
	for _, entry := range strings.Split(alpn, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" || len(entry) > 255 {
			continue
		}
		b = append(b, byte(len(entry)))
		b = append(b, []byte(entry)...)
	}
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

// quanxTLSOn emits the protocol family's TLS switch plus its SNI carrier.
func quanxTLSOn(obfsTLS bool, sni string) []string {
	if obfsTLS {
		out := []string{"obfs=over-tls"}
		if sni != "" {
			out = append(out, "obfs-host="+quoteValue(sni))
		}
		return out
	}
	out := []string{"over-tls=true"}
	if sni != "" {
		out = append(out, "tls-host="+quoteValue(sni))
	}
	return out
}

// quanxTransport renders QX's obfs / TLS parameters for one node. sample.conf
// documents only these transports: plain TCP (optionally wrapped by the
// family's TLS switch, reality keys, tls-verification and tls-alpn),
// WebSocket (obfs=ws, obfs=wss with obfs-host/obfs-uri) and plain HTTP
// (obfs=http). grpc / xhttp / quic / kcp links have no QX representation, so
// the second return is false and the caller degrades the node to a comment
// instead of a silently broken line.
func quanxTransport(n *shareNode) ([]string, bool) {
	sec := n.Params["security"]
	switch n.Params["type"] {
	case "", "tcp", "none":
	case "ws", "http":
	default:
		return nil, false
	}
	// Trojan is TLS-only by design; sample.conf never shows it without
	// over-tls (or reality keys), and never shows anytls off tcp.
	if n.Protocol == "trojan" && sec != "tls" && sec != "reality" {
		return nil, false
	}
	if n.Protocol == "anytls" && n.Params["type"] != "" && n.Params["type"] != "tcp" && n.Params["type"] != "none" {
		return nil, false
	}
	// sample.conf's trojan note: obfs is only supported with websocket over tls.
	if n.Protocol == "trojan" && n.Params["type"] == "ws" && sec != "tls" && sec != "reality" {
		return nil, false
	}
	obfsTLS := quanxObfsTLS(n.Protocol)
	switch n.Params["type"] {
	case "ws":
		obfs := "ws"
		if sec == "tls" || sec == "reality" {
			obfs = "wss"
		}
		out := []string{"obfs=" + obfs}
		if h := shareParam(n.Params, "host", "sni"); h != "" {
			out = append(out, "obfs-host="+quoteValue(h))
		}
		if p := shareParam(n.Params, "path"); p != "" {
			out = append(out, "obfs-uri="+quoteValue(p))
		}
		if sec == "reality" {
			out = append(out, quanxReality(n.Params)...)
		}
		return out, true
	case "http":
		// sample.conf documents obfs=http on plain (non-TLS) lines only.
		if sec == "tls" || sec == "reality" {
			return nil, false
		}
		out := []string{"obfs=http"}
		if h := shareParam(n.Params, "host", "sni"); h != "" {
			out = append(out, "obfs-host="+quoteValue(h))
		}
		if p := shareParam(n.Params, "path"); p != "" {
			out = append(out, "obfs-uri="+quoteValue(p))
		}
		return out, true
	}
	// Plain TCP: the family's TLS switch, plus reality keys or the
	// verification/alpn pair sample.conf shows next to it.
	switch sec {
	case "reality":
		out := quanxTLSOn(obfsTLS, shareParam(n.Params, "sni"))
		return append(out, quanxReality(n.Params)...), true
	case "tls":
		out := quanxTLSOn(obfsTLS, shareParam(n.Params, "sni"))
		out = append(out, "tls-verification="+strconv.FormatBool(!shareInsecure(n.Params)))
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			if h := quanxAlpnHex(alpn); h != "" {
				out = append(out, "tls-alpn="+h)
			}
		}
		return out, true
	}
	return nil, true
}

func quanxSSPlugin(plugin string) []string {
	out := []string{}
	if plugin == "" {
		return out
	}
	obfs, host, uri := "", "", ""
	for _, seg := range strings.Split(plugin, ";") {
		seg = strings.TrimSpace(seg)
		if strings.EqualFold(seg, "obfs-local") {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		switch strings.ToLower(k) {
		case "obfs":
			obfs = v
		case "obfs-host":
			host = v
		case "obfs-uri":
			uri = v
		}
	}
	if obfs != "" && obfs != "none" {
		out = append(out, "obfs="+obfs)
	}
	if host != "" {
		out = append(out, "obfs-host="+host)
	}
	if uri != "" {
		out = append(out, "obfs-uri="+quoteValue(uri))
	}
	return out
}

// ---- public generators ----

// clientShareLinks returns the decoded share links (one URI per line) for a
// proxy user: bound listeners, credentials and remote mirrors, using exactly
// the same resolution as the base64 subscription.
func clientShareLinks(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]string, error) {
	raw, err := GenerateUserBase64Subscription(db, pu, req, gen)
	if err != nil {
		return nil, err
	}
	body := raw
	if decoded, decErr := base64.StdEncoding.DecodeString(string(raw)); decErr == nil {
		body = decoded
	}
	var links []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			links = append(links, line)
		}
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("no exportable share links for user")
	}
	return links, nil
}

// appleClientTarget selects the target grammar for the shared generator.
type appleClientTarget struct {
	client  string                          // loon / surge / quanx
	render  func(*shareNode) string         // node-line renderer
	header  []string                        // section header lines
	footer  func(names []string) []string   // optional trail (Surge group)
	skipMsg func(*shareNode) (string, bool) // optional unsupported-protocol reason
}

func generateAppleClientSubscription(t appleClientTarget, db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if !user.IsCredentialActive(pu) {
		return nil, fmt.Errorf("user is not active")
	}
	links, err := clientShareLinks(db, pu, req, gen)
	if err != nil {
		return nil, err
	}

	lines := append([]string{}, t.header...)
	names := make([]string, 0, len(links))
	seen := map[string]int{}
	for _, link := range links {
		node, perr := parseShareURI(link)
		if perr != nil {
			lines = append(lines, fmt.Sprintf("# unparsable share link skipped: %v", perr))
			continue
		}
		name := node.Name
		if n, dup := seen[name]; dup {
			seen[name] = n + 1
			name = fmt.Sprintf("%s-%d", name, n+1)
			node.Name = name
		} else {
			seen[name] = 1
		}
		if t.skipMsg != nil {
			if reason, unsupported := t.skipMsg(node); unsupported {
				lines = append(lines, commentLine(name, reason))
				continue
			}
		}
		line := t.render(node)
		if line == "" {
			lines = append(lines, commentLine(name, t.client+" renderer could not represent this node"))
			continue
		}
		// Only nodes that actually rendered may be referenced by footer
		// sections (e.g. Surge's [Proxy Group]); dangling policy names make
		// real clients reject the whole profile.
		names = append(names, name)
		lines = append(lines, line)
	}
	if t.footer != nil {
		lines = append(lines, t.footer(names)...)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// GenerateUserLoonSubscription exports the user's share links as a Loon
// subscription ([Proxy] section, one node per line).
// GenerateUserLoonSubscription exports the user's share links as a Loon
// subscription ([Proxy] section, one node per line).
func GenerateUserLoonSubscription(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]byte, error) {
	return generateAppleClientSubscription(appleClientTarget{
		client:  "loon",
		render:  renderLoonNode,
		header:  []string{"[Proxy]"},
		skipMsg: loonSkipMsg,
	}, db, pu, req, gen)
}

// surgeVMessOK reports whether a VMess encryption method has a Surge
// counterpart. policies/vmess.html: "Optional, `aes-128-gcm` or
// `chacha20-ietf-poly1305`, default: `aes-128-gcm`". An empty value means
// the optional parameter was not supplied, so Surge's default applies.
// The share-link value `auto` is deliberately NOT accepted: it has no
// documented Surge representation and degrades to a comment.
func surgeVMessOK(method string) bool {
	m := strings.ToLower(strings.TrimSpace(method))
	return m == "" || surgeVMessMethods[m]
}

// surgeVLESSUnsupported reports VLESS combinations Surge cannot express,
// per manual.nssurge.com/policies/vless.html (Surge iOS 6.0.0+ / Mac 6.12.0+).
//
// Supported combinations documented on that page:
//
//	Transport   Security            Vision flow
//	RAW (TCP)   none, TLS, REALITY   With TLS or REALITY
//	WebSocket   none, TLS            No
//	gRPC        TLS, REALITY         No
//
// "XHTTP, HTTPUpgrade and mKCP transports and VLESS Encryption are not
// supported." Vision requires TLS 1.3 (tls=true or REALITY) and cannot be
// used with WebSocket; gRPC requires tls=true or REALITY. REALITY is enabled
// by reality-public-key, so a reality link without pbk cannot be expressed.
// Closed value sets (flow, reality-short-id, grpc-mode, ws-path) are also
// checked here so no undocumented value can be emitted.
// surgeVLESSUnsupported reports VLESS combinations Surge cannot express,
// per manual.nssurge.com/policies/vless.html (Surge iOS 6.0.0+ /
// Mac 6.12.0+). The four-row matrix is transcribed verbatim from that page:
//
//	Transport   Security            Vision flow
//	RAW (TCP)   none, TLS, REALITY   With TLS, REALITY or VLESS Encryption
//	WebSocket   none, TLS            With VLESS Encryption
//	gRPC        TLS, REALITY         No
//	XHTTP       none, TLS, REALITY   With VLESS Encryption
//
// "VLESS Encryption can be added to any of these combinations. HTTPUpgrade
// and mKCP transports are not supported." (verbatim) - neither XHTTP nor
// VLESS Encryption is unsupported; both are implemented by renderSurgeNode.
//
// Closed value sets (flow, reality-short-id, grpc-mode, xhttp-mode, ws-path,
// encryption) are checked here so no undocumented value can be emitted.
func surgeVLESSUnsupported(n *shareNode) (string, bool) {
	p := n.Params
	typ := strings.ToLower(strings.TrimSpace(p["type"]))
	switch typ {
	case "", "tcp", "raw", "none", "ws", "grpc", "xhttp", "splithttp":
	default:
		// Only HTTPUpgrade and mKCP are ruled out by the manual.
		return "Surge VLESS 不支持该传输（HTTPUpgrade / mKCP 不受支持）", true
	}
	// encryption: "Optional, default: none". A real VLESS Encryption value is
	// the mlkem768x25519plus.* client string printed by `xray vlessenc`.
	enc := strings.TrimSpace(p["encryption"])
	if enc != "" && !strings.EqualFold(enc, "none") &&
		!strings.HasPrefix(enc, surgeVLESSEncPrefix) {
		return "Surge VLESS encryption 值不符合官方 mlkem768x25519plus.* 格式", true
	}
	hasEnc := enc != "" && !strings.EqualFold(enc, "none")
	// "flow: Optional, xtls-rprx-vision or xtls-rprx-vision-udp443".
	flow := strings.TrimSpace(p["flow"])
	if flow != "" && !inSet(surgeVLESSFlows, flow) {
		return "Surge VLESS flow 仅支持 xtls-rprx-vision / xtls-rprx-vision-udp443", true
	}
	// "reality-short-id: up to 16 hexadecimal digits".
	if sid := strings.TrimSpace(p["sid"]); !surgeShortIDOK(sid) {
		return "Surge VLESS reality-short-id 须为不超过 16 位十六进制", true
	}
	// The share-link `mode` field maps to grpc-mode for type=grpc and to
	// xhttp-mode for type=xhttp; each has its own documented closed set.
	mode := strings.TrimSpace(p["mode"])
	switch typ {
	case "grpc":
		if mode != "" && !inSet(surgeGRPCModes, mode) {
			return "Surge VLESS grpc-mode 仅支持 gun / multi", true
		}
	case "xhttp", "splithttp":
		if mode != "" && !inSet(surgeVLESSXHTTPModes, mode) {
			return "Surge VLESS xhttp-mode 仅支持 auto / packet-up / stream-up / stream-one", true
		}
	}
	// "ws-path ... It must start with `/`" (inherited from the VMess page).
	// The XHTTP page states no such requirement for xhttp-path, so none is
	// imposed here.
	if typ == "ws" {
		if pth := strings.TrimSpace(p["path"]); pth != "" && !wsPathOK(pth) {
			return "Surge ws-path 须以 / 开头", true
		}
	}
	sec := strings.ToLower(strings.TrimSpace(p["security"]))
	reality := sec == "reality" || strings.TrimSpace(p["pbk"]) != ""
	tls := sec == "tls"
	switch typ {
	case "ws":
		// WebSocket security is none or TLS only.
		if reality {
			return "Surge VLESS 的 WebSocket 不支持 REALITY", true
		}
		// Vision over WebSocket requires VLESS Encryption.
		if flow != "" && !hasEnc {
			return "Surge VLESS 的 Vision flow 在 WebSocket 上需 VLESS Encryption", true
		}
	case "xhttp", "splithttp":
		// XHTTP security is none, TLS or REALITY; Vision needs Encryption.
		if flow != "" && !hasEnc {
			return "Surge VLESS 的 Vision flow 在 XHTTP 上需 VLESS Encryption", true
		}
	case "grpc":
		// gRPC requires TLS or REALITY and never supports Vision.
		if !tls && !reality {
			return "Surge VLESS 的 gRPC 需 TLS 或 REALITY", true
		}
		if flow != "" {
			return "Surge VLESS 的 Vision flow 不支持 gRPC", true
		}
	default: // RAW / TCP: Vision needs TLS, REALITY or VLESS Encryption.
		if flow != "" && !tls && !reality && !hasEnc {
			return "Surge VLESS 的 Vision flow 需 TLS / REALITY / VLESS Encryption", true
		}
	}
	if reality && strings.TrimSpace(p["pbk"]) == "" {
		return "Surge VLESS 的 REALITY 缺 reality-public-key", true
	}
	return "", false
}

// surgeSkipMsg reports nodes Surge cannot express. Every rejection points at
// a documented constraint, and the renderer calls this too, so a value
// outside a documented closed set can never appear on a policy line.
func surgeSkipMsg(n *shareNode) (string, bool) {
	switch n.Protocol {
	case "vless":
		return surgeVLESSUnsupported(n)
	case "trojan", "vmess":
		// policies/trojan.html and policies/vmess.html document no
		// reality-public-key / reality-short-id parameters.
		if strings.EqualFold(n.Params["security"], "reality") {
			return "Surge 不支持 REALITY 传输（trojan / vmess 无 reality-public-key）", true
		}
		if !surgeTransportOK(n.Params["type"]) {
			return "Surge 仅支持 WS 传输（trojan / vmess）", true
		}
		if pth := shareParam(n.Params, "path"); pth != "" && !wsPathOK(pth) {
			return "Surge ws-path 须以 / 开头", true
		}
		if n.Protocol == "vmess" && !surgeVMessOK(n.Method) {
			return "Surge VMess 仅支持 aes-128-gcm / chacha20-ietf-poly1305 加密", true
		}
	case "ss":
		if !inSet(surgeSSMethods, n.Method) {
			return "Surge SS 加密方式不在文档支持列表", true
		}
		if o := ssPluginObfs(n.Params["plugin"]); o != "" && o != "none" && !inSet(surgeObfsModes, o) {
			return "Surge SS obfs 仅支持 http / tls", true
		}
	case "hysteria2":
		if o := shareParam(n.Params, "obfs"); o != "" && !inSet(surgeHy2Obfs, o) {
			return "Surge Hysteria2 混淆仅支持 salamander / gecko", true
		}
	case "anytls":
		// policies/anytls.html documents only `password` and `reuse`; there
		// is no transport parameter, so a non-TCP link cannot be expressed
		// without silently dropping the transport.
		if t := strings.ToLower(strings.TrimSpace(n.Params["type"])); t != "" && t != "tcp" && t != "none" {
			return "Surge AnyTLS 仅支持 TCP（文档只有 password / reuse）", true
		}
	}
	return "", false
}

// GenerateUserSurgeSubscription exports the user's share links as a Surge
// profile ([General]/[Proxy]/[Proxy Group]/[Rule] mini profile with a select
// policy that points at every node).
func GenerateUserSurgeSubscription(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]byte, error) {
	return generateAppleClientSubscription(appleClientTarget{
		client:  "surge",
		render:  renderSurgeNode,
		header:  []string{"[General]", "", "[Proxy]"},
		skipMsg: surgeSkipMsg,
		footer: func(names []string) []string {
			group := "3m-ui"
			kept := make([]string, 0, len(names))
			for _, name := range names {
				kept = append(kept, quoteValue(name))
			}
			return []string{
				"",
				"[Proxy Group]",
				fmt.Sprintf("%s = select, DIRECT, %s", group, strings.Join(kept, ", ")),
				"",
				"[Rule]",
				"FINAL," + group,
			}
		},
	}, db, pu, req, gen)
}

// GenerateUserQuanXSubscription exports the user's share links as a Quantumult
// X subscription ([server_local] section, one node per line).
func GenerateUserQuanXSubscription(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]byte, error) {
	return generateAppleClientSubscription(appleClientTarget{
		client: "quanx",
		render: renderQuanXNode,
		header: []string{"[server_local]"},
		skipMsg: func(n *shareNode) (string, bool) {
			switch n.Protocol {
			case "hysteria2":
				return "Quantumult X 不支持 Hysteria2 协议", true
			case "tuic":
				return "Quantumult X 不支持 TUIC 协议", true
			}
			return "", false
		},
	}, db, pu, req, gen)
}
