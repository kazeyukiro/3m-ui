package config

// ListenerSchema is the protocol-specific allowlist used at the backend trust boundary.
type ListenerSchema struct {
	Protocol     string
	Fields       map[string]struct{}
	NestedFields map[string]map[string]struct{}
}

func listenerFields(values ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}

func listenerNested(values ...string) map[string]map[string]struct{} {
	m := make(map[string]map[string]struct{})
	for _, value := range values {
		p := splitPath(value)
		for i := 0; i < len(p)-1; i++ {
			parent := joinPath(p[:i+1])
			if m[parent] == nil {
				m[parent] = map[string]struct{}{}
			}
			m[parent][p[i+1]] = struct{}{}
		}
	}
	return m
}

func splitPath(v string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == '.' {
			if i > start {
				out = append(out, v[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func joinPath(p []string) string {
	s := ""
	for i, v := range p {
		if i > 0 {
			s += "."
		}
		s += v
	}
	return s
}

// Mihomo listener schemas mirror the official listener examples. These are
// the fields exposed at the node configuration boundary; protocol-local
// capture endpoints such as socks/http/tun/tproxy are intentionally excluded.
//
// Design note (P3-5): the "users" field is intentionally present only at the
// top level (Fields), never in NestedFields. Per-protocol the "users" shape
// differs: vmess/vless/trojan/anytls use an array of objects
// [{username,uuid,...}], whereas hysteria2/tuic-v5/mieru use a map of strings
// {username: password}. The validator's array recursion (see C2 fix in
// listener_validation.go) already accepts both shapes without recursing into
// per-user keys. Adding users.<key> to NestedFields would break map-form
// validation, so per-user field allowlisting is deferred as a follow-up.
//
// Design note (R-M4, fix-G-schema): vmess/vless/trojan Fields include 5
// grpc-opts extended keys (grpc-user-agent / ping-interval / max-connections /
// min-streams / max-streams) as panel-side metadata for client YAML
// generation. Per proxies-transport wiki these are CLIENT-only fields — the
// listener side has no grpc-opts block, only flat top-level keys. The panel's
// free-form JSON editor stores them on the listener JSON; converter/client.go
// copyTransport grpc case reads them and emits them into the client-side
// grpc-opts block. Validation is HARD-reject (see listener_validation.go), so
// the schema MUST whitelist them or the panel will reject listener JSON that
// carries them. copyConfigPassthrough in protocol/compile.go forwards them to
// listener config.yaml — mihomo silently ignores unknown fields, so the leak
// is harmless. A coordinated compile.go edit (adding the 5 keys to
// clientOnlyListenerKey) would strip them from listener YAML output — flagged
// as follow-up (out of fix-G-schema scope; requires touching protocol/compile.go).
//
// Design note (R-M3, fix-G-schema): vmess NestedFields mekya-config block
// whitelists polling-interval-initial and h2-pool-size (per proxies-transport
// mekya-opts wiki). Same client-metadata pattern as R-M4: the listener side
// stores them inside mekya-config; converter/client.go copyTransport mekya-config
// case reads them and emits into client-side mekya-opts. The listener
// mekya-config block is emitted verbatim by copyConfigPassthrough — mihomo's
// listener mekya-config implementation silently ignores these 2 keys (only
// enable / max-write-* / packet-writing-buffer / kcp are consumed).
var MihomoListenerSchemas = map[string]ListenerSchema{
	"shadowsocks": {
		Protocol: "shadowsocks",
		Fields: listenerFields(
			"cipher", "password", "udp", "simple-obfs", "shadow-tls", "res-tls", "jls-config", "kcp-tun", "mux-option",
		),
		NestedFields: listenerNested(
			"simple-obfs.enable", "simple-obfs.mode",
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
			"kcp-tun.enable", "kcp-tun.key", "kcp-tun.crypt", "kcp-tun.mode", "kcp-tun.conn", "kcp-tun.autoexpire", "kcp-tun.scavengettl", "kcp-tun.ratelimit", "kcp-tun.mtu", "kcp-tun.sndwnd", "kcp-tun.rcvwnd", "kcp-tun.datashard", "kcp-tun.parityshard", "kcp-tun.dscp", "kcp-tun.nocomp", "kcp-tun.acknodelay", "kcp-tun.nodelay", "kcp-tun.interval", "kcp-tun.resend", "kcp-tun.sockbuf", "kcp-tun.smuxver", "kcp-tun.smuxbuf", "kcp-tun.framesize", "kcp-tun.streambuf", "kcp-tun.keepalive",
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
		),
	},
	"snell": {
		Protocol: "snell",
		Fields:   listenerFields("psk", "version", "udp", "obfs-opts", "shadow-tls", "res-tls", "jls-config"),
		NestedFields: listenerNested(
			"obfs-opts.mode", "obfs-opts.host",
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
		),
	},
	"vmess": {
		Protocol: "vmess",
		Fields:   listenerFields("users", "alterId", "ws-path", "grpc-service-name", "grpc-user-agent", "ping-interval", "max-connections", "min-streams", "max-streams", "mekya-config", "mkcp-config", "shadow-tls", "res-tls", "jls-config", "reality-config", "tlsmirror-config", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "allow-insecure", "mux-option"),
		NestedFields: listenerNested(
			"mekya-config.enable", "mekya-config.max-write-size", "mekya-config.max-write-duration-ms", "mekya-config.max-simultaneous-write-connection", "mekya-config.packet-writing-buffer", "mekya-config.polling-interval-initial", "mekya-config.h2-pool-size",
			"mekya-config.kcp.mtu", "mekya-config.kcp.tti", "mekya-config.kcp.uplink-capacity", "mekya-config.kcp.downlink-capacity", "mekya-config.kcp.congestion", "mekya-config.kcp.write-buffer", "mekya-config.kcp.read-buffer", "mekya-config.kcp.seed", "mekya-config.kcp.header",
			"mkcp-config.enable", "mkcp-config.mtu", "mkcp-config.tti", "mkcp-config.uplink-capacity", "mkcp-config.downlink-capacity", "mkcp-config.congestion", "mkcp-config.write-buffer", "mkcp-config.read-buffer", "mkcp-config.seed", "mkcp-config.header",
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
			"reality-config.dest", "reality-config.private-key", "reality-config.short-id", "reality-config.server-names", "reality-config.max-time-difference", "reality-config.proxy", "reality-config.limit-fallback-upload.after-bytes", "reality-config.limit-fallback-upload.bytes-per-sec", "reality-config.limit-fallback-upload.burst-bytes-per-sec", "reality-config.limit-fallback-download.after-bytes", "reality-config.limit-fallback-download.bytes-per-sec", "reality-config.limit-fallback-download.burst-bytes-per-sec",
			"tlsmirror-config.dest", "tlsmirror-config.primary-key", "tlsmirror-config.proxy", "tlsmirror-config.explicit-nonce-ciphersuites", "tlsmirror-config.defer-instance-derived-write-time.base-nanoseconds", "tlsmirror-config.defer-instance-derived-write-time.uniform-random-multiplier-nanoseconds", "tlsmirror-config.transport-layer-padding.enabled", "tlsmirror-config.connection-enrolment.primary-ingress-outbound", "tlsmirror-config.sequence-watermarking-enabled",
			"tlsmirror-config.embedded-traffic-generator.steps.name", "tlsmirror-config.embedded-traffic-generator.steps.host", "tlsmirror-config.embedded-traffic-generator.steps.path", "tlsmirror-config.embedded-traffic-generator.steps.method", "tlsmirror-config.embedded-traffic-generator.steps.headers", "tlsmirror-config.embedded-traffic-generator.steps.connection-ready", "tlsmirror-config.embedded-traffic-generator.steps.connection-recall-exit", "tlsmirror-config.embedded-traffic-generator.steps.h2-do-not-wait-for-download-finish", "tlsmirror-config.embedded-traffic-generator.steps.wait-time.base-nanoseconds", "tlsmirror-config.embedded-traffic-generator.steps.wait-time.uniform-random-multiplier-nanoseconds", "tlsmirror-config.embedded-traffic-generator.steps.next-step.weight", "tlsmirror-config.embedded-traffic-generator.steps.next-step.goto-location",
		),
	},
	"vless": {
		Protocol: "vless",
		Fields:   listenerFields("users", "flow", "ws-path", "grpc-service-name", "grpc-user-agent", "ping-interval", "max-connections", "min-streams", "max-streams", "xhttp-config", "decryption", "encryption", "reality-config", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "allow-insecure", "jls-config", "shadow-tls", "res-tls", "mux-option"),
		NestedFields: listenerNested(
			"xhttp-config.path", "xhttp-config.host", "xhttp-config.mode", "xhttp-config.no-sse-header", "xhttp-config.x-padding-bytes", "xhttp-config.x-padding-obfs-mode", "xhttp-config.x-padding-key", "xhttp-config.x-padding-header", "xhttp-config.x-padding-placement", "xhttp-config.x-padding-method", "xhttp-config.uplink-http-method", "xhttp-config.session-placement", "xhttp-config.session-key", "xhttp-config.session-table", "xhttp-config.session-length", "xhttp-config.seq-placement", "xhttp-config.seq-key", "xhttp-config.uplink-data-placement", "xhttp-config.uplink-data-key", "xhttp-config.uplink-chunk-size", "xhttp-config.sc-max-buffered-posts", "xhttp-config.sc-stream-up-server-secs", "xhttp-config.sc-max-each-post-bytes",
			"reality-config.dest", "reality-config.private-key", "reality-config.short-id", "reality-config.server-names", "reality-config.max-time-difference", "reality-config.proxy", "reality-config.limit-fallback-upload.after-bytes", "reality-config.limit-fallback-upload.bytes-per-sec", "reality-config.limit-fallback-upload.burst-bytes-per-sec", "reality-config.limit-fallback-download.after-bytes", "reality-config.limit-fallback-download.bytes-per-sec", "reality-config.limit-fallback-download.burst-bytes-per-sec",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
		),
	},
	"trojan": {
		Protocol: "trojan",
		Fields:   listenerFields("users", "ws-path", "grpc-service-name", "grpc-user-agent", "ping-interval", "max-connections", "min-streams", "max-streams", "reality-config", "ss-option", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "allow-insecure", "shadow-tls", "res-tls", "jls-config", "mux-option"),
		NestedFields: listenerNested(
			"reality-config.dest", "reality-config.private-key", "reality-config.short-id", "reality-config.server-names", "reality-config.max-time-difference", "reality-config.proxy", "reality-config.limit-fallback-upload.after-bytes", "reality-config.limit-fallback-upload.bytes-per-sec", "reality-config.limit-fallback-upload.burst-bytes-per-sec", "reality-config.limit-fallback-download.after-bytes", "reality-config.limit-fallback-download.bytes-per-sec", "reality-config.limit-fallback-download.burst-bytes-per-sec",
			"ss-option.enabled", "ss-option.method", "ss-option.password",
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
		),
	},
	"hysteria2": {
		Protocol: "hysteria2",
		Fields: listenerFields(
			"users", "up", "down", "ignore-client-bandwidth", "obfs", "obfs-password", "masquerade", "realm-opts", "bbr-profile", "alpn", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "mux-option",
		),
		NestedFields: listenerNested(
			"realm-opts.enable", "realm-opts.server-url", "realm-opts.token", "realm-opts.realm-id", "realm-opts.stun-servers", "realm-opts.proxy", "realm-opts.sni", "realm-opts.skip-cert-verify", "realm-opts.fingerprint", "realm-opts.certificate", "realm-opts.private-key", "realm-opts.alpn", "realm-opts.name-cert-verify",
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
		),
	},
	"tuic": {
		Protocol: "tuic",
		Fields: listenerFields(
			"users", "token", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "congestion-controller", "bbr-profile", "max-idle-time", "authentication-timeout", "alpn", "max-udp-relay-packet-size", "mux-option",
		),
		NestedFields: listenerNested(
			"mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down",
		),
	},
	"tuic-v4": {
		Protocol:     "tuic-v4",
		Fields:       listenerFields("users", "token", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "congestion-controller", "bbr-profile", "max-idle-time", "authentication-timeout", "alpn", "max-udp-relay-packet-size", "mux-option"),
		NestedFields: listenerNested("mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down"),
	},
	"tuic-v5": {
		Protocol:     "tuic-v5",
		Fields:       listenerFields("users", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "congestion-controller", "bbr-profile", "max-idle-time", "authentication-timeout", "alpn", "max-udp-relay-packet-size", "mux-option"),
		NestedFields: listenerNested("mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down"),
	},
	"shadowquic": {
		Protocol:     "shadowquic",
		Fields:       listenerFields("users", "jls-upstream", "alpn", "quic-versions", "zero-rtt", "congestion-controller", "up", "down", "ignore-client-bandwidth", "cwnd", "bbr-profile", "max-idle-time", "max-datagram-frame-size", "recv-window-conn", "recv-window", "disable-mtu-discovery"),
		NestedFields: listenerNested("jls-upstream.addr", "jls-upstream.sni", "jls-upstream.proxy", "jls-upstream.rate-limit"),
	},
	"anytls": {
		Protocol: "anytls",
		Fields:   listenerFields("users", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "shadow-tls", "res-tls", "jls-config", "allow-insecure", "padding-scheme"),
		NestedFields: listenerNested(
			"shadow-tls.enable", "shadow-tls.version", "shadow-tls.password", "shadow-tls.users", "shadow-tls.handshake.dest", "shadow-tls.handshake.proxy",
			"res-tls.enable", "res-tls.dest", "res-tls.password", "res-tls.restls-script", "res-tls.min-record-len", "res-tls.proxy", "res-tls.rate-limit",
			"jls-config.enable", "jls-config.users", "jls-config.dest", "jls-config.sni", "jls-config.alpn", "jls-config.proxy", "jls-config.rate-limit",
		),
	},
	"mieru": {
		Protocol: "mieru",
		Fields:   listenerFields("transport", "users", "traffic-pattern", "user-hint-is-mandatory"),
	},
	"sudoku": {
		Protocol:     "sudoku",
		Fields:       listenerFields("key", "aead-method", "padding-min", "padding-max", "table-type", "custom-table", "custom-tables", "handshake-timeout", "enable-pure-downlink", "httpmask", "fallback", "disable-http-mask", "http-mask-mode", "path-root", "mux-option"),
		NestedFields: listenerNested("httpmask.disable", "httpmask.mode", "httpmask.path-root", "mux-option.padding", "mux-option.brutal", "mux-option.brutal.enabled", "mux-option.brutal.up", "mux-option.brutal.down"),
	},
	"trusttunnel": {
		Protocol: "trusttunnel",
		Fields:   listenerFields("users", "certificate", "private-key", "client-auth-type", "client-auth-cert", "ech-key", "network", "congestion-controller", "bbr-profile"),
	},
}

func GetMihomoListenerSchema(protocol string) (ListenerSchema, bool) {
	schema, ok := MihomoListenerSchemas[protocol]
	return schema, ok
}
