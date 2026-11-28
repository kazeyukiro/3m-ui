package converter

import (
	"strings"
	"testing"
)

// Golden node lines captured from live Sub-Store 2.44.31 (commit d0bf093)
// /api/proxy/parse, feeding the exact share-link URIs below. The alignment is
// byte-for-byte on the node line. Sub-Store emits only node lines; the panel's
// section headers ([Proxy]/[server_local]/...) are retained as the subscription
// skeleton (Sub-Store's own front-end adds headers), and unsupported protocols
// are dropped silently by omitSkipComments.
var ssGoldenURIs = []string{
	"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388#ss-plain",
	"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dbing.com#ss-obfs",
	"vmess://eyJ2IjoiMiIsInBzIjoidm1lc3Mtd3MiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjExMTExMTExLTExMTEtNDExMS04MTExLTExMTExMTExMTExMSIsIm5ldCI6IndzIiwidHlwZSI6Im5vbmUiLCJob3N0Ijoidm0uZXhhbXBsZS5jb20iLCJwYXRoIjoiL3dzIiwidGxzIjoidHJ1ZSIsInNuaSI6InZtLmV4YW1wbGUuY29tIn0=#vmess-ws",
	"vless://11111111-1111-4111-8111-111111111111@vless.example.com:443?security=tls&sni=vless.example.com&type=ws&path=%2Fws&host=vless.example.com&insecure=1#vless-ws-insecure",
	"vless://11111111-1111-4111-8111-111111111111@vless.example.com:443?security=reality&pbk=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk&sid=0123456789abcdef&sni=www.microsoft.com&flow=xtls-rprx-vision&type=tcp#vless-reality",
	"trojan://trojan-pw@trojan.example.com:443?sni=trojan.example.com&alpn=h2%2Chttp%2F1.1#trojan-alpn",
	"hysteria2://hy2-pw@hy2.example.com:443?sni=hy2.example.com&alpn=h3&mport=5000-6000&insecure=1#hy2-hop-insecure",
	"anytls://anytls-pw@anytls.example.com:443?sni=anytls.example.com&alpn=h3#anytls-alpn",
	"tuic://11111111-1111-4111-8111-111111111111:tuic-pw@tuic.example.com:443?sni=tuic.example.com&alpn=h3#tuic-v5",
}

func ssGoldenLines(t *testing.T, render func(*shareNode) string, skip func(*shareNode) (string, bool), want []string) {
	t.Helper()
	var got []string
	for _, uri := range ssGoldenURIs {
		n, err := parseShareURI(uri)
		if err != nil {
			t.Fatalf("parse %q: %v", uri, err)
		}
		if skip != nil {
			if _, unsupported := skip(n); unsupported {
				continue
			}
		}
		line := render(n)
		if line != "" {
			got = append(got, line)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("node count = %d, want %d\ngot:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("node %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestLoonSSAlignedWithSubStore(t *testing.T) {
	ssGoldenLines(t, renderLoonNodeSS, ssLoonSkipMsg, []string{
		`ss-plain=shadowsocks,ss.example.com,8388,aes-128-gcm,"password",udp=true`,
		`ss-obfs=shadowsocks,ss.example.com,8388,aes-128-gcm,"password",obfs-name=http,obfs-host=bing.com,udp=true`,
		`vmess-ws=vmess,vm.example.com,443,auto,"11111111-1111-4111-8111-111111111111",transport=ws,path=/ws,host=vm.example.com,over-tls=false,alterId=0,udp=true`,
		`vless-ws-insecure=vless,vless.example.com,443,"11111111-1111-4111-8111-111111111111",transport=ws,path=/ws,host=vless.example.com,over-tls=true,skip-cert-verify=true,tls-name=vless.example.com,udp=true`,
		`vless-reality=vless,vless.example.com,443,"11111111-1111-4111-8111-111111111111",transport=tcp,over-tls=true,skip-cert-verify=false,flow=xtls-rprx-vision,sni=www.microsoft.com,public-key="k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk",short-id=0123456789abcdef,udp=true`,
		`trojan-alpn=trojan,trojan.example.com,443,"trojan-pw",alpn="h2,http/1.1",tls-name=trojan.example.com,udp=true`,
		`hy2-hop-insecure=Hysteria2,hy2.example.com,443,"hy2-pw",server-ports="5000-6000",tls-name=hy2.example.com,skip-cert-verify=true,fast-open=false,udp=true`,
		`anytls-alpn=anytls,anytls.example.com,443,"anytls-pw",skip-cert-verify=false,alpn="h3",tls-name=anytls.example.com,udp=true`,
	})
}

func TestSurgeSSAlignedWithSubStore(t *testing.T) {
	ssGoldenLines(t, renderSurgeNodeSS, ssSurgeSkipMsg, []string{
		`ss-plain=ss,ss.example.com,8388,encrypt-method=aes-128-gcm,password="password",udp-relay=true`,
		`ss-obfs=ss,ss.example.com,8388,encrypt-method=aes-128-gcm,password="password",obfs=http,obfs-host=bing.com,udp-relay=true`,
		`vmess-ws=vmess,vm.example.com,443,username=11111111-1111-4111-8111-111111111111,ws=true,ws-path=/ws,ws-headers="Host:"vm.example.com"",vmess-aead=true,tls=false`,
		`vless-ws-insecure=vless,vless.example.com,443,username=11111111-1111-4111-8111-111111111111,ws=true,ws-path=/ws,ws-headers="Host:"vless.example.com"",tls=true,sni="vless.example.com",skip-cert-verify=true`,
		`vless-reality=vless,vless.example.com,443,username=11111111-1111-4111-8111-111111111111,flow=xtls-rprx-vision,reality-public-key=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk,reality-short-id=0123456789abcdef,sni="www.microsoft.com"`,
		`trojan-alpn=trojan,trojan.example.com,443,password="trojan-pw",tls=true,sni="trojan.example.com",alpn="h2,http/1.1"`,
		`hy2-hop-insecure=hysteria2,hy2.example.com,443,password="hy2-pw",port-hopping="5000-6000",sni="hy2.example.com",skip-cert-verify=true,tfo=false`,
		`anytls-alpn=anytls,anytls.example.com,443,password="anytls-pw",sni="anytls.example.com",alpn="h3",skip-cert-verify=false`,
		`tuic-v5=tuic-v5,tuic.example.com,443,uuid=11111111-1111-4111-8111-111111111111,password="tuic-pw",sni="tuic.example.com",alpn="h3"`,
	})
}

func TestQuanXSSAlignedWithSubStore(t *testing.T) {
	ssGoldenLines(t, renderQuanXNodeSS, ssQXSkipMsg, []string{
		`shadowsocks=ss.example.com:8388,method=aes-128-gcm,password=password,udp-relay=true,tag=ss-plain`,
		`shadowsocks=ss.example.com:8388,method=aes-128-gcm,password=password,obfs=http,obfs-host=bing.com,udp-relay=true,tag=ss-obfs`,
		`vmess=vm.example.com:443,method=chacha20-poly1305,password=11111111-1111-4111-8111-111111111111,obfs=ws,obfs-uri=/ws,obfs-host=vm.example.com,aead=true,udp-relay=true,tag=vmess-ws`,
		`vless=vless.example.com:443,method=none,password=11111111-1111-4111-8111-111111111111,obfs=wss,obfs-uri=/ws,obfs-host=vless.example.com,tls-verification=false,tls-host=vless.example.com,udp-relay=true,tag=vless-ws-insecure`,
		`vless=vless.example.com:443,method=none,password=11111111-1111-4111-8111-111111111111,obfs=over-tls,tls-verification=true,tls-host=www.microsoft.com,vless-flow=xtls-rprx-vision,udp-relay=true,tag=vless-reality,reality-base64-pubkey=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk,reality-hex-shortid=0123456789abcdef`,
		`trojan=trojan.example.com:443,password=trojan-pw,over-tls=true,tls-alpn=02683208687474702f312e31,tls-host=trojan.example.com,udp-relay=true,tag=trojan-alpn`,
		`hysteria2=hy2.example.com:443,password=hy2-pw,port-hopping=5000-6000,tls-host=hy2.example.com,tls-verification=false,udp-relay=true,tag=hy2-hop-insecure`,
		`anytls=anytls.example.com:443,password=anytls-pw,over-tls=true,tls-alpn=026833,tls-verification=true,tls-host=anytls.example.com,udp-relay=true,tag=anytls-alpn`,
	})
}
