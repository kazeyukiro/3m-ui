package realityscan

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
)

var reservedRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2002::/16"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range reservedRanges {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

const (
	addressFallbackDelay = 200 * time.Millisecond
	addressDialTimeout   = time.Second
	maxAddressDials      = 2
)

// Pin DNS validation to the address actually dialed; never use HTTP proxies or
// follow redirects into internal networks. The API accepts no arbitrary targets.
func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{}
	return publicDialWith(ctx, network, address, net.DefaultResolver.LookupNetIP, dialer.DialContext)
}

func publicDialWith(ctx context.Context, network, address string,
	lookup func(context.Context, string, string) ([]netip.Addr, error),
	dial func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := lookup(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, fmt.Errorf("non-public target address")
		}
	}
	ips = interleaveAddresses(ips)

	// Try the other address family promptly, including when the first route is a
	// blackhole. At most two literal IPs are dialed at once. Each attempt is also
	// bounded so two bad routes cannot consume the entire five-second probe.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type dialResult struct {
		conn net.Conn
		err  error
	}
	results := make(chan dialResult)
	next, active := 0, 0
	start := func() {
		ip := ips[next]
		next++
		active++
		go func() {
			attemptCtx, stop := context.WithTimeout(ctx, addressDialTimeout)
			defer stop()
			conn, err := dial(attemptCtx, network, net.JoinHostPort(ip.String(), port))
			select {
			case results <- dialResult{conn: conn, err: err}:
			case <-ctx.Done():
				// A late successful connection belongs to this losing attempt.
				if conn != nil {
					conn.Close()
				}
			}
		}()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start()
	fallback := time.NewTicker(addressFallbackDelay)
	defer fallback.Stop()
	for active > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			active--
			if err := ctx.Err(); err != nil {
				if result.conn != nil {
					result.conn.Close()
				}
				return nil, err
			}
			if result.err == nil {
				return result.conn, nil
			}
			err = result.err
			if next < len(ips) {
				start()
			}
		case <-fallback.C:
			if active < maxAddressDials && next < len(ips) {
				start()
			}
		}
	}
	return nil, err
}

// Preserve the resolver's preferred family while avoiding a whole block of
// unreachable IPv6 addresses delaying the first IPv4 attempt (or vice versa).
func interleaveAddresses(ips []netip.Addr) []netip.Addr {
	preferred, alternate := []netip.Addr{}, []netip.Addr{}
	seen := make(map[netip.Addr]bool, len(ips))
	prefer4 := ips[0].Unmap().Is4()
	for _, ip := range ips {
		ip = ip.Unmap()
		if seen[ip] {
			continue
		}
		seen[ip] = true
		if ip.Is4() == prefer4 {
			preferred = append(preferred, ip)
		} else {
			alternate = append(alternate, ip)
		}
	}
	out := make([]netip.Addr, 0, len(seen))
	for i := 0; i < max(len(preferred), len(alternate)); i++ {
		if i < len(preferred) {
			out = append(out, preferred[i])
		}
		if i < len(alternate) {
			out = append(out, alternate[i])
		}
	}
	return out
}

func cloudflareHeaders(h http.Header) bool {
	return h.Get("Cf-Ray") != "" || strings.Contains(strings.ToLower(h.Get("Server")), "cloudflare")
}

func sharedDeliveryHeaders(h http.Header) string {
	if cloudflareHeaders(h) {
		return "cloudflare"
	}
	if h.Get("X-Amz-Cf-Id") != "" || h.Get("X-Amz-Cf-Pop") != "" || strings.Contains(strings.ToLower(h.Get("Via")), "cloudfront.net") {
		return "cloudfront"
	}
	// This pair is a conservative Fastly-style cache signal, not proof that the
	// endpoint accepts other tenants' SNI. Do not reject generic X-Cache alone.
	if h.Get("X-Served-By") != "" && h.Get("X-Timer") != "" {
		return "fastly_style_cache"
	}
	if strings.Contains(strings.ToLower(h.Get("Server")), "akamai") {
		return "akamai"
	}
	return ""
}

func cloudflareTrace(body string) bool {
	fields := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && value != "" {
			fields[key] = true
		}
	}
	return fields["colo"] && fields["ip"] && fields["h"] && fields["ts"]
}

// Conservative automatic-selection exclusions for recognizable delivery
// networks. This does not prove domain-fronting support, and missing signals
// do not prove dedicated hosting. Host labels alone (e.g. "cdn") are not used.
func sharedDeliveryCNAME(cname string) bool {
	cname = strings.ToLower(strings.TrimSuffix(cname, "."))
	for _, suffix := range []string{"cloudflare.net", "fastly.net", "cloudfront.net", "edgekey.net", "edgesuite.net", "akamaiedge.net", "akadns.net", "azureedge.net", "cdnmix.net"} {
		if cname == suffix || strings.HasSuffix(cname, "."+suffix) {
			return true
		}
	}
	return false
}

func probeTarget(ctx context.Context, host string) Result {
	cname, err := net.DefaultResolver.LookupCNAME(ctx, host)
	if err != nil {
		return Result{Target: net.JoinHostPort(host, "443"), ServerName: host, Reason: "dns_check_failed"}
	}
	if sharedDeliveryCNAME(cname) {
		return Result{Target: net.JoinHostPort(host, "443"), ServerName: host, Reason: "shared_delivery_cname", Evidence: Evidence{CNAME: cname}}
	}
	transport := &http.Transport{
		DialContext: publicDial, ForceAttemptHTTP2: true,
		MaxResponseHeaderBytes: 32 * 1024,
		TLSClientConfig: &tls.Config{
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.X25519, tls.X25519MLKEM768},
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r := probeHTTPS(ctx, client, "https://"+host, host)
	r.Evidence.CNAME = cname
	return r
}

func probeHTTPS(ctx context.Context, client *http.Client, baseURL, host string) Result {
	r := Result{Target: net.JoinHostPort(host, "443"), ServerName: host}
	start := time.Now()
	var latency atomic.Int64
	var remoteIP atomic.Value
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		host, _, _ := net.SplitHostPort(info.Conn.RemoteAddr().String())
		remoteIP.Store(host)
	}, TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
		if err == nil {
			latency.Store(time.Since(start).Milliseconds())
		}
	}}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodHead, baseURL+"/", nil)
	if err != nil {
		r.Reason = "request_failed"
		return r
	}
	resp, err := client.Do(req)
	if err != nil {
		r.Reason = "connection_or_certificate_failed"
		return r
	}
	resp.Body.Close()
	r.LatencyMS = int(latency.Load())
	r.Evidence.HTTPStatus = resp.StatusCode
	r.Evidence.Server = resp.Header.Get("Server")
	r.Evidence.Redirect = resp.Header.Get("Location")
	r.Evidence.Cloudflare = cloudflareHeaders(resp.Header)
	r.Evidence.DeliverySignal = sharedDeliveryHeaders(resp.Header)
	if ip, ok := remoteIP.Load().(string); ok {
		r.Evidence.IP = ip
	}
	if state := resp.TLS; state != nil {
		r.Evidence.TLSVersion = tls.VersionName(state.Version)
		r.Evidence.ALPN = state.NegotiatedProtocol
		r.Evidence.KeyExchange = state.CurveID.String()
		r.Evidence.CertificateValid = len(state.VerifiedChains) > 0
		if len(state.PeerCertificates) > 0 {
			r.Evidence.CertificateExpires = state.PeerCertificates[0].NotAfter.UTC().Format(time.RFC3339)
		}
	}
	switch {
	case cloudflareHeaders(resp.Header):
		r.Reason = "cloudflare_detected"
	case r.Evidence.DeliverySignal != "":
		r.Reason = "shared_delivery_headers"
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		r.Reason = "http_redirect"
	case resp.StatusCode < 200 || resp.StatusCode >= 400:
		r.Reason = "http_unavailable"
	case resp.TLS == nil || resp.TLS.Version != tls.VersionTLS13:
		r.Reason = "tls13_required"
	case resp.TLS.NegotiatedProtocol != "h2":
		r.Reason = "h2_required"
	case resp.TLS.CurveID != tls.X25519 && resp.TLS.CurveID != tls.X25519MLKEM768:
		r.Reason = "x25519_required"
	case len(resp.TLS.VerifiedChains) == 0:
		r.Reason = "certificate_untrusted"
	}
	if r.Reason != "" {
		return r
	}
	// A bounded Cloudflare-specific signal, not a general CDN safety proof.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/cdn-cgi/trace", nil)
	if err != nil {
		r.Reason = "request_failed"
		return r
	}
	resp, err = client.Do(req)
	if err != nil {
		r.Reason = "risk_check_failed"
		return r
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		r.Reason = "risk_check_failed"
		return r
	}
	if cloudflareHeaders(resp.Header) || cloudflareTrace(string(body)) {
		r.Evidence.Cloudflare = true
		r.Reason = "cloudflare_detected"
		return r
	}
	if signal := sharedDeliveryHeaders(resp.Header); signal != "" {
		r.Evidence.DeliverySignal = signal
		r.Reason = "shared_delivery_headers"
		return r
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		r.Reason = "risk_check_redirect"
		return r
	}
	r.Eligible = true
	return r
}
