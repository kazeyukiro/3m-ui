package converter

import (
	"fmt"
	"strconv"
	"strings"
)

// This file implements the Sub-Store 2.44.31 node-line rendering for the
// Apple/iOS targets (Loon, Surge, Quantumult X). It is a separate renderer so
// the previously verified "1.4.7 compatible" renderers in client_apple.go stay
// intact and the subscription entry points can switch between them.
//
// Every field, its spelling and its order below were derived from two sources,
// cross-checked against each other (no invented parameters):
//   1. live Sub-Store 2.44.31 (commit d0bf093) /api/proxy/parse output, feeding
//      the exact same share-link URIs the panel exports; and
//   2. the producer source: backend/src/core/proxy-utils/producers/{loon,
//      surge,qx}.js and the URI parser defaults in parsers/index.js.
//
// The input path that matters here is URI -> producer (the panel exports
// share links). Note: the `tls-profile=chrome` field seen in earlier audits is
// produced only on the Clash-YAML input path, not from URIs, so it is absent
// from these renderers.

// ssJoinNodeLine renders "NAME=proto,server,port,…" (Sub-Store uses '=' with
// no surrounding spaces, unlike the documented renderer's ' = ').
func ssJoinNodeLine(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + "=" + strings.Join(parts[1:], ",")
}

// ssQuote wraps a value in double quotes unconditionally (Sub-Store's
// producers quote the credential position: `"password"`, `"uuid"`).
func ssQuote(v string) string {
	return `"` + strings.ReplaceAll(v, `"`, "") + `"`
}

// ssRawQuote wraps in double quotes without stripping inner quotes. Sub-Store's
// Surge ws-headers path quotes the header value twice: formatHeaderMap quotes
// the value, then quoteSurgeValue quotes the whole "Host:\"...\"" string,
// producing e.g. ws-headers="Host:"vm.example.com"". This is reproduced
// verbatim for strict byte-level alignment.
func ssRawQuote(v string) string {
	return `"` + v + `"`
}

// ssWrappedQuote quotes only when needed (comma / space / quote), matching
// Sub-Store's formatLoonValue behaviour for non-credential values.
func ssWrappedQuote(v string) string {
	if strings.ContainsAny(v, ",;\" ") {
		return ssQuote(v)
	}
	return v
}

// ssProtoName maps the panel's internal protocol name to the keyword Sub-Store
// writes. From the producers: shadowsocks (lowercase), vless, trojan, anytls,
// vmess are lowercase; Hysteria2 keeps its capital H.
func ssProtoName(proto string) string {
	switch proto {
	case "ss":
		return "shadowsocks"
	case "hysteria2":
		return "Hysteria2"
	default:
		return proto
	}
}

// ssAlpnHex mirrors Sub-Store's encodeQxAlpn: each ALPN token is length-prefixed
// (one hex byte) followed by its UTF-8 hex bytes. "h3" -> "026833".
func ssAlpnHex(alpn string) string {
	tokens := strings.Split(alpn, ",")
	var b strings.Builder
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		fmt.Fprintf(&b, "%02x", len(tok))
		for _, r := range tok {
			fmt.Fprintf(&b, "%02x", r)
		}
	}
	return b.String()
}

// ssInsecure mirrors the parser's skip-cert-verify assignment. Sub-Store's
// URI parsers set skip-cert-verify on vless/vmess/anytls/hysteria2 even when
// false; trojan's parser sets it only when insecure. The producers then emit
// it via appendIfPresent.
func ssSkipCertValue(params map[string]string) (string, bool) {
	if shareInsecure(params) {
		return "true", true
	}
	return "false", true
}

// ---- Loon (Sub-Store alignment) ------------------------------------------

// ssLoonSkipMsg mirrors Sub-Store's Loon producer protocol support.
func ssLoonSkipMsg(n *shareNode) (string, bool) {
	switch n.Protocol {
	case "tuic":
		return "Loon 暂不支持 TUIC 协议", true
	case "mieru":
		return "Loon 暂不支持 Mieru 协议", true
	case "http", "socks5", "socks":
		return "Loon 暂不支持 HTTP/SOCKS 协议", true
	}
	return "", false
}

// renderLoonNodeSS renders one node in Sub-Store's Loon grammar.
func renderLoonNodeSS(n *shareNode) string {
	switch n.Protocol {
	case "hysteria2":
		parts := []string{n.Name, "Hysteria2", n.Host, n.Port, ssQuote(n.Password)}
		// server-ports: Sub-Store emits proxy.ports verbatim (comma ranges);
		// the panel carries hops in mport with dash ranges.
		if hop := shareParam(n.Params, "mport", "ports"); hop != "" {
			// Sub-Store emits the hop range verbatim (dash form, e.g. 5000-6000).
			parts = append(parts, `server-ports="`+strings.ReplaceAll(hop, `"`, "")+`"`)
		}
		if sni := shareParam(n.Params, "sni", "peer"); sni != "" {
			parts = append(parts, "tls-name="+sni)
		}
		if v, ok := ssSkipCertValue(n.Params); ok {
			parts = append(parts, "skip-cert-verify="+v)
		}
		// Sub-Store's hysteria2 URI parser never sets proxy.alpn, so no
		// alpn field is emitted even when the link carries alpn=h3.
		// Sub-Store's parser sets tfo=false by default for hysteria2 -> fast-open=false.
		parts = append(parts, "fast-open=false")
		parts = append(parts, "udp=true")
		return ssJoinNodeLine(parts)
	case "ss":
		if n.Method == "" {
			return ""
		}
		parts := []string{n.Name, "shadowsocks", n.Host, n.Port, n.Method, ssQuote(n.Password)}
		parts = append(parts, loonSSPlugin(n.Params["plugin"])...)
		parts = append(parts, "udp=true")
		return ssJoinNodeLine(parts)
	case "vmess":
		parts := []string{n.Name, "vmess", n.Host, n.Port, "auto", ssQuote(n.Password)}
		transport := "tcp"
		path, host := "", ""
		switch n.Params["type"] {
		case "ws", "http":
			transport = "ws"
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		case "xhttp":
			transport = "xhttp"
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		}
		parts = append(parts, "transport="+transport)
		if path != "" {
			parts = append(parts, "path="+path)
		}
		if host != "" {
			parts = append(parts, "host="+host)
		}
		overTLS := "false"
		if n.Params["security"] == "tls" || n.Params["security"] == "reality" {
			overTLS = "true"
		}
		parts = append(parts, "over-tls="+overTLS)
		// Sub-Store's vmess parser sets skip-cert-verify only when insecure.
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		parts = append(parts, "alterId=0", "udp=true")
		return ssJoinNodeLine(parts)
	case "vless":
		parts := []string{n.Name, "vless", n.Host, n.Port, ssQuote(n.Password)}
		transport := "tcp"
		path, host := "", ""
		flow := shareParam(n.Params, "flow")
		switch n.Params["type"] {
		case "ws", "http":
			transport = "ws"
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
		case "xhttp":
			transport = "xhttp"
			path = shareParam(n.Params, "path")
			host = shareParam(n.Params, "host")
			flow = ""
		}
		parts = append(parts, "transport="+transport)
		if path != "" {
			parts = append(parts, "path="+path)
		}
		if host != "" {
			parts = append(parts, "host="+host)
		}
		overTLS := "false"
		security := strings.ToLower(strings.TrimSpace(n.Params["security"]))
		if security == "tls" || security == "reality" {
			overTLS = "true"
		}
		parts = append(parts, "over-tls="+overTLS)
		if v, ok := ssSkipCertValue(n.Params); ok {
			parts = append(parts, "skip-cert-verify="+v)
		}
		if flow != "" {
			parts = append(parts, "flow="+flow)
		}
		if security == "reality" {
			if sni := shareParam(n.Params, "sni"); sni != "" {
				parts = append(parts, "sni="+sni)
			}
			if pbk := shareParam(n.Params, "pbk"); pbk != "" {
				parts = append(parts, "public-key="+ssQuote(pbk))
			}
			if sid := shareParam(n.Params, "sid"); sid != "" {
				parts = append(parts, "short-id="+sid)
			}
		} else if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-name="+sni)
		}
		parts = append(parts, "udp=true")
		return ssJoinNodeLine(parts)
	case "trojan":
		parts := []string{n.Name, "trojan", n.Host, n.Port, ssQuote(n.Password)}
		// Sub-Store trojan: transport only when ws/xhttp; tcp is deleted.
		switch n.Params["type"] {
		case "ws", "http":
			parts = append(parts, "transport=ws")
			if p := shareParam(n.Params, "path"); p != "" {
				parts = append(parts, "path="+p)
			}
			if h := shareParam(n.Params, "host"); h != "" {
				parts = append(parts, "host="+h)
			}
		}
		// trojan parser sets skip-cert-verify only when insecure.
		if shareInsecure(n.Params) {
			parts = append(parts, "skip-cert-verify=true")
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+ssQuote(alpn))
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-name="+sni)
		}
		parts = append(parts, "udp=true")
		return ssJoinNodeLine(parts)
	case "anytls":
		parts := []string{n.Name, "anytls", n.Host, n.Port, ssQuote(n.Password)}
		if v, ok := ssSkipCertValue(n.Params); ok {
			parts = append(parts, "skip-cert-verify="+v)
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+ssQuote(alpn))
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-name="+sni)
		}
		parts = append(parts, "udp=true")
		return ssJoinNodeLine(parts)
	}
	return ""
}

// ---- Surge (Sub-Store alignment) -----------------------------------------

func ssSurgeSkipMsg(n *shareNode) (string, bool) {
	switch n.Protocol {
	case "mieru", "sudoku", "shadowquic", "trusttunnel":
		return "Surge 不支持该协议", true
	}
	return "", false
}

// renderSurgeNodeSS renders one node in Sub-Store's Surge grammar.
func renderSurgeNodeSS(n *shareNode) string {
	switch n.Protocol {
	case "hysteria2":
		parts := []string{n.Name, "hysteria2", n.Host, n.Port, "password=" + ssQuote(n.Password)}
		if hop := shareParam(n.Params, "mport", "ports"); hop != "" {
			parts = append(parts, "port-hopping="+ssQuote(strings.ReplaceAll(hop, ",", ";")))
		}
		if sni := shareParam(n.Params, "sni", "peer"); sni != "" {
			parts = append(parts, "sni="+ssQuote(sni))
		}
		if v, ok := ssSkipCertValue(n.Params); ok {
			parts = append(parts, "skip-cert-verify="+v)
		}
		parts = append(parts, "tfo=false")
		return ssJoinNodeLine(parts)
	case "ss":
		if n.Method == "" {
			return ""
		}
		parts := []string{n.Name, "ss", n.Host, n.Port, "encrypt-method=" + strings.ToLower(n.Method), "password=" + ssQuote(n.Password)}
		parts = append(parts, surgeSSPlugin(n.Params["plugin"])...)
		parts = append(parts, "udp-relay=true")
		return ssJoinNodeLine(parts)
	case "vmess":
		params := []string{"username=" + n.Password}
		if m := strings.ToLower(strings.TrimSpace(n.Method)); surgeVMessMethods[m] {
			params = append(params, "encrypt-method="+m)
		}
		if n.Params["type"] == "ws" {
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+p)
			}
			if h := shareParam(n.Params, "host"); h != "" {
				params = append(params, "ws-headers="+ssRawQuote("Host:"+ssQuote(h)))
			}
		}
		// Sub-Store's Surge vmess producer always emits vmess-aead=true and
		// tls=<bool> for URI input.
		params = append(params, "vmess-aead=true")
		tls := "false"
		if n.Params["security"] == "tls" || n.Params["security"] == "reality" {
			tls = "true"
		}
		params = append(params, "tls="+tls)
		if sni := shareParam(n.Params, "sni"); sni != "" {
			params = append(params, "sni="+ssQuote(sni))
		}
		// Sub-Store's vmess parser sets skip-cert-verify only when insecure.
		if shareInsecure(n.Params) {
			params = append(params, "skip-cert-verify=true")
		}
		parts := append([]string{n.Name, "vmess", n.Host, n.Port}, params...)
		return ssJoinNodeLine(parts)
	case "vless":
		params := []string{"username=" + n.Password}
		security := strings.ToLower(strings.TrimSpace(n.Params["security"]))
		reality := security == "reality" || strings.TrimSpace(n.Params["pbk"]) != ""
		if flow := shareParam(n.Params, "flow"); flow != "" {
			params = append(params, "flow="+flow)
		}
		switch strings.ToLower(strings.TrimSpace(n.Params["type"])) {
		case "", "tcp", "raw", "none":
		case "ws":
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+p)
			}
			if h := shareParam(n.Params, "host"); h != "" {
				params = append(params, "ws-headers="+ssRawQuote("Host:"+ssQuote(h)))
			}
		case "grpc":
			params = append(params, "grpc=true")
			if sn := shareParam(n.Params, "serviceName"); sn != "" {
				params = append(params, "grpc-service-name="+ssQuote(sn))
			}
		case "xhttp", "splithttp":
			params = append(params, "xhttp=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "xhttp-path="+ssQuote(p))
			}
		}
		if reality {
			if pbk := shareParam(n.Params, "pbk"); pbk != "" {
				params = append(params, "reality-public-key="+pbk)
			}
			if sid := shareParam(n.Params, "sid"); sid != "" {
				params = append(params, "reality-short-id="+sid)
			}
		} else if security == "tls" {
			params = append(params, "tls=true")
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			params = append(params, "sni="+ssQuote(sni))
		}
		// Surge's vless page: with REALITY skip-cert-verify does not apply.
		if !reality {
			if v, ok := ssSkipCertValue(n.Params); ok {
				params = append(params, "skip-cert-verify="+v)
			}
		}
		parts := append([]string{n.Name, "vless", n.Host, n.Port}, params...)
		return ssJoinNodeLine(parts)
	case "trojan":
		params := []string{"password=" + ssQuote(n.Password)}
		if n.Params["type"] == "ws" {
			params = append(params, "ws=true")
			if p := shareParam(n.Params, "path"); p != "" {
				params = append(params, "ws-path="+p)
			}
		}
		// Trojan always TLS; Sub-Store emits tls=true and quoted sni.
		params = append(params, "tls=true")
		if sni := shareParam(n.Params, "sni"); sni != "" {
			params = append(params, "sni="+ssQuote(sni))
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts := append([]string{n.Name, "trojan", n.Host, n.Port}, params...)
			parts = append(parts, "alpn="+ssQuote(alpn))
			return ssJoinNodeLine(parts)
		}
		parts := append([]string{n.Name, "trojan", n.Host, n.Port}, params...)
		return ssJoinNodeLine(parts)
	case "anytls":
		parts := []string{n.Name, "anytls", n.Host, n.Port, "password=" + ssQuote(n.Password)}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "sni="+ssQuote(sni))
		}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+ssQuote(alpn))
		}
		if v, ok := ssSkipCertValue(n.Params); ok {
			parts = append(parts, "skip-cert-verify="+v)
		}
		return ssJoinNodeLine(parts)
	case "tuic":
		// Sub-Store's URI_TUIC parser splits userinfo on ':' into
		// uuid:password and the Surge producer always emits tuic-v5 with
		// both fields. A token-only link (no ':') becomes uuid=<token>,
		// password="". The panel's shareNode stores uuid:password as
		// User:Password, but a token-only link as Password alone.
		uuid, pw := n.User, n.Password
		if n.User == "" {
			uuid, pw = n.Password, ""
		}
		parts := []string{n.Name, "tuic-v5", n.Host, n.Port, "uuid=" + uuid, "password=" + ssQuote(pw)}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "sni="+ssQuote(sni))
		}
		alpn := shareParam(n.Params, "alpn")
		if alpn == "" {
			alpn = "h3"
		}
		parts = append(parts, "alpn="+ssQuote(alpn))
		return ssJoinNodeLine(parts)
	}
	return ""
}

// ---- Quantumult X (Sub-Store alignment) ----------------------------------

func ssQXSkipMsg(n *shareNode) (string, bool) {
	switch n.Protocol {
	case "tuic":
		return "Quantumult X 不支持 TUIC 协议", true
	}
	return "", false
}

// renderQuanXNodeSS renders one node in Sub-Store's QX [server_local] grammar.
func renderQuanXNodeSS(n *shareNode) string {
	head := fmt.Sprintf("%s=%s:%s", quanxProtoKeyword(n.Protocol), n.Host, n.Port)
	switch n.Protocol {
	case "hysteria2":
		parts := []string{head}
		if n.Password == "" {
			return ""
		}
		parts = append(parts, "password="+n.Password)
		if hop := shareParam(n.Params, "mport", "ports"); hop != "" {
			// Sub-Store emits the hop range verbatim (dash form).
			parts = append(parts, "port-hopping="+hop)
		}
		if sni := shareParam(n.Params, "sni", "peer"); sni != "" {
			parts = append(parts, "tls-host="+sni)
		}
		// Sub-Store's hysteria2 parser never sets proxy.alpn; only an
		// explicit tls-alpn parameter would be emitted by appendTlsAlpn.
		if talpn := shareParam(n.Params, "tls-alpn"); talpn != "" {
			parts = append(parts, "tls-alpn="+talpn)
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "tls-verification=false")
		}
		parts = append(parts, "udp-relay=true", "tag="+n.Name)
		return strings.Join(parts, ",")
	case "ss":
		if n.Method == "" {
			return ""
		}
		parts := []string{head, "method=" + n.Method, "password=" + n.Password}
		parts = append(parts, quanxSSPlugin(n.Params["plugin"])...)
		parts = append(parts, "udp-relay=true", "tag="+n.Name)
		return strings.Join(parts, ",")
	case "vmess":
		// Sub-Store's formatQXVmessMethod falls back to chacha20-poly1305
		// for auto/none/empty; only aes-128-gcm has a distinct value.
		method := "chacha20-poly1305"
		switch strings.ToLower(strings.TrimSpace(n.Method)) {
		case "aes-128-gcm":
			method = "aes-128-gcm"
		}
		parts := []string{head, "method=" + method, "password=" + n.Password}
		if n.Params["type"] == "ws" || n.Params["type"] == "http" {
			parts = append(parts, "obfs=ws")
			if p := shareParam(n.Params, "path"); p != "" {
				parts = append(parts, "obfs-uri="+p)
			}
			if h := shareParam(n.Params, "host"); h != "" {
				parts = append(parts, "obfs-host="+h)
			}
		}
		parts = append(parts, "aead=true", "udp-relay=true", "tag="+n.Name)
		return strings.Join(parts, ",")
	case "vless":
		parts := []string{head, "method=none", "password=" + n.Password}
		security := strings.ToLower(strings.TrimSpace(n.Params["security"]))
		reality := security == "reality" || strings.TrimSpace(n.Params["pbk"]) != ""
		if n.Params["type"] == "ws" || n.Params["type"] == "http" {
			parts = append(parts, "obfs=wss")
			if p := shareParam(n.Params, "path"); p != "" {
				parts = append(parts, "obfs-uri="+p)
			}
			if h := shareParam(n.Params, "host"); h != "" {
				parts = append(parts, "obfs-host="+h)
			}
			if shareInsecure(n.Params) {
				parts = append(parts, "tls-verification=false")
			} else {
				parts = append(parts, "tls-verification=true")
			}
		} else {
			parts = append(parts, "obfs=over-tls")
			if shareInsecure(n.Params) {
				parts = append(parts, "tls-verification=false")
			} else {
				parts = append(parts, "tls-verification=true")
			}
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-host="+sni)
		}
		if flow := shareParam(n.Params, "flow"); flow != "" {
			parts = append(parts, "vless-flow="+flow)
		}
		parts = append(parts, "udp-relay=true", "tag="+n.Name)
		if reality {
			if pbk := shareParam(n.Params, "pbk"); pbk != "" {
				parts = append(parts, "reality-base64-pubkey="+pbk)
			}
			if sid := shareParam(n.Params, "sid"); sid != "" {
				parts = append(parts, "reality-hex-shortid="+sid)
			}
		}
		return strings.Join(parts, ",")
	case "trojan":
		parts := []string{head, "password=" + n.Password, "over-tls=true"}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "tls-alpn="+ssAlpnHex(alpn))
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-host="+sni)
		}
		parts = append(parts, "udp-relay=true", "tag="+n.Name)
		return strings.Join(parts, ",")
	case "anytls":
		parts := []string{head, "password=" + n.Password, "over-tls=true"}
		if alpn := shareParam(n.Params, "alpn"); alpn != "" {
			parts = append(parts, "tls-alpn="+ssAlpnHex(alpn))
		}
		if shareInsecure(n.Params) {
			parts = append(parts, "tls-verification=false")
		} else {
			parts = append(parts, "tls-verification=true")
		}
		if sni := shareParam(n.Params, "sni"); sni != "" {
			parts = append(parts, "tls-host="+sni)
		}
		parts = append(parts, "udp-relay=true", "tag="+n.Name)
		return strings.Join(parts, ",")
	}
	return ""
}

// keep strconv referenced for future numeric field handling without importing
// it conditionally. (alpn length prefixes are rendered by ssAlpnHex.)
var _ = strconv.Itoa
