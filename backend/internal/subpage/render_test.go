package subpage

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestParseNotices(t *testing.T) {
	raw := strings.Join([]string{
		"# 注释行应被忽略",
		"",
		"*09-15|机房升级通知|9 月 15 日 02:00-04:00 升级，期间可能短暂中断",
		"10-20|新增洛杉矶 LA-01 节点",
		"|只有标题没有日期",
		"仅标题",
		"*|置顶无日期",
	}, "\n")
	got := parseNotices(raw)
	if len(got) != 5 {
		t.Fatalf("want 5 notices, got %d: %+v", len(got), got)
	}
	if !got[0].Pinned || got[0].Date != "09-15" || got[0].Title != "机房升级通知" {
		t.Fatalf("notice[0] = %+v", got[0])
	}
	if !strings.Contains(got[0].Desc, "短暂中断") {
		t.Fatalf("notice[0].desc = %q", got[0].Desc)
	}
	if got[1].Pinned || got[1].Date != "10-20" || got[1].Desc != "" {
		t.Fatalf("notice[1] = %+v", got[1])
	}
	if got[2].Date != "" || got[2].Title != "只有标题没有日期" {
		t.Fatalf("notice[2] = %+v", got[2])
	}
	if got[3].Title != "仅标题" {
		t.Fatalf("notice[3] = %+v", got[3])
	}
	if !got[4].Pinned || got[4].Title != "置顶无日期" {
		t.Fatalf("notice[4] = %+v", got[4])
	}
}

func TestParseNoticesCapsAndCount(t *testing.T) {
	long := strings.Repeat("x", maxNoticeTitleLen+50)
	got := parseNotices("|" + long)
	if len(got) != 1 {
		t.Fatalf("want 1 notice, got %d", len(got))
	}
	if len([]rune(got[0].Title)) > maxNoticeTitleLen {
		t.Fatalf("title not capped: %d runes", len([]rune(got[0].Title)))
	}
	// Multi-byte runes must not be split at the cap.
	zh := strings.Repeat("公", maxNoticeTitleLen+10)
	got = parseNotices("|" + zh)
	if len([]rune(got[0].Title)) != maxNoticeTitleLen {
		t.Fatalf("zh title cap: %d runes", len([]rune(got[0].Title)))
	}
	// Entry count cap.
	var lines []string
	for i := 0; i < maxNotices+10; i++ {
		lines = append(lines, "t"+string(rune('a'+i%26)))
	}
	if n := len(parseNotices(strings.Join(lines, "\n"))); n != maxNotices {
		t.Fatalf("want %d notices, got %d", maxNotices, n)
	}
}

func TestParseNoticesMultilinePaste(t *testing.T) {
	// The editor format is one notice per line: a pasted multi-line block
	// becomes multiple notices (newlines inside a field are impossible).
	got := parseNotices("|标题|第一行\n第二行")
	if len(got) != 2 {
		t.Fatalf("want 2 notices, got %d: %+v", len(got), got)
	}
	if got[0].Title != "标题" || got[0].Desc != "第一行" {
		t.Fatalf("notice[0] = %+v", got[0])
	}
	if got[1].Title != "第二行" || got[1].Desc != "" {
		t.Fatalf("notice[1] = %+v", got[1])
	}
}

func TestFormatNoticesRoundTrip(t *testing.T) {
	raw := "*09-15|机房升级通知|02:00-04:00 升级\n10-20|新增节点\n|无日期标题\n*|置顶"
	first := formatNotices(parseNotices(raw))
	second := formatNotices(parseNotices(first))
	if first != second {
		t.Fatalf("not stable:\n%q\n%q", first, second)
	}
	back := parseNotices(first)
	if len(back) != 4 || !back[0].Pinned || back[1].Pinned || !back[3].Pinned {
		t.Fatalf("round trip lost pin flags: %+v", back)
	}
	if back[2].Date != "" || back[2].Title != "无日期标题" {
		t.Fatalf("round trip lost empty date: %+v", back[2])
	}
}

func TestUriDerivedFields(t *testing.T) {
	cases := []struct {
		uri  string
		name string
		meta string
		key  string
	}{
		{"vless://uuid@h:443?encryption=none&security=tls&type=grpc#HK-01", "HK-01", "gRPC · TLS", "vless"},
		{"vmess://eyJ2IjoiMiJ9", "", "", "vmess"},
		{"trojan://pw@h:443?security=tls#SG-03", "SG-03", "TLS", "trojan"},
		{"hysteria2://pw@h:443?sni=h#LA-01", "LA-01", "QUIC", "hy2"},
		{"ss://YWVz@h:8388#DE%2D01", "DE-01", "", "ss"},
		{"tuic://uuid:pw@h:443#TK-01", "TK-01", "QUIC", "tuic"},
		{"wireguard://k@h:51820#WG", "WG", "WireGuard", "wg"},
		{"anytls://pw@h:443#AT", "AT", "TLS", "anytls"},
		{"vless://uuid@h:80?type=ws&security=none#WS-01", "WS-01", "WebSocket", "vless"},
		{"not a uri", "", "", "other"},
	}
	for _, c := range cases {
		if got := uriFragmentName(c.uri); got != c.name {
			t.Errorf("uriFragmentName(%q) = %q, want %q", c.uri, got, c.name)
		}
		if got := uriMeta(c.uri); got != c.meta {
			t.Errorf("uriMeta(%q) = %q, want %q", c.uri, got, c.meta)
		}
		if got := protocolKey(c.uri); got != c.key {
			t.Errorf("protocolKey(%q) = %q, want %q", c.uri, got, c.key)
		}
	}
}

// testViewModel returns a fully populated ViewModel covering every field the
// default template reads.
func testViewModel() ViewModel {
	return ViewModel{
		Username:       "audituser",
		TrafficUsedH:   "42.6 GiB",
		TrafficLimitH:  "200.00 GiB",
		TrafficPct:     21.3,
		RemainBytesH:   "157.40 GiB",
		UploadBytesH:   "12.40 GiB",
		DownloadBytesH: "30.20 GiB",
		ExpireTime:     "2027-03-14T00:00:00Z",
		IPLimit:        3,
		SubURL:         "https://sub.example.com/api/v1/client/sub/tok",
		SubV2RayURL:    "https://sub.example.com/api/v1/client/sub/tok?target=v2ray",
		SubTitle:       "3m-ui Subscription",
		PageTitle:      "3m-ui 订阅",
		SubSupportURL:  "https://t.me/example",
		SubQRDataURI:   template.URL("data:image/png;base64,QUJD"),
		NodeCount:      2,
		HasExpiry:      true,
		DaysLeft:       140,
		Notices: []NoticeItem{
			{Date: "09-15", Title: "机房升级通知", Desc: "02:00-04:00", Pinned: true},
			{Date: "10-20", Title: "新增节点"},
		},
		URIItems: []URIItem{
			{Protocol: "VLESS", URI: "vless://u@h:443?security=tls&type=grpc#HK-01", Name: "HK-01", Meta: "gRPC · TLS", ProtocolKey: "vless", Index: 1},
			{Protocol: "SS", URI: "ss://YWVz@h:8388#DE-01", Name: "DE-01", ProtocolKey: "ss", Index: 2},
		},
		ClashDeepLink:   template.URL("clash://install-config?url=https%3A%2F%2Fsub.example.com%2Fapi%2Fv1%2Fclient%2Fsub%2Ftok%3Ftarget%3Dclash"),
		SingboxDeepLink: template.URL("sing-box://import-remote?url=https%3A%2F%2Fsub.example.com%2Fapi%2Fv1%2Fclient%2Fsub%2Ftok%3Ftarget%3Dsingbox"),
	}
}

// renderTestPage renders the default template with testViewModel() and
// returns the raw HTML bytes.
func renderTestPage(t *testing.T) []byte {
	t.Helper()
	tpl, err := loadTemplate("")
	if err != nil {
		t.Fatalf("loadTemplate: %v", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, testViewModel()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.Bytes()
}

func TestDefaultTemplateRenders(t *testing.T) {
	out := string(renderTestPage(t))
	must := []string{
		"3m-ui 订阅",
		"audituser",
		`data:image/png;base64,QUJD`,
		"clash://install-config?url=https%3A%2F%2Fsub.example.com",
		"sing-box://import-remote?url=https%3A%2F%2Fsub.example.com",
		"公告",
		"机房升级通知",
		"02:00-04:00",
		"新增节点",
		"HK-01",
		"gRPC · TLS",
		"157.40 GiB",
		"2027-03-14T00:00:00Z",
		"12.40 GiB / 30.20 GiB",
		"https://t.me/example",
		"vless://u@h:443?security=tls&amp;type=grpc#HK-01",
		`data-link="vless://u@h:443?security=tls&amp;type=grpc#HK-01"`,
		`data-link="https://sub.example.com/api/v1/client/sub/tok"`,
		`data-days="140"`,
		`data-n="2"`,
		`width: 21.30%`,
	}
	for _, m := range must {
		if !strings.Contains(out, m) {
			t.Errorf("output missing %q", m)
		}
	}
	// Removed mock concepts must not exist anymore.
	for _, bad := range []string{"旗舰年付", "服务在线", "class=\"live\"", "class='live'"} {
		if strings.Contains(out, bad) {
			t.Errorf("output still contains %q", bad)
		}
	}
	// Pinned notice renders expanded with the pin tag.
	if !strings.Contains(out, `class="notice-item open"`) || !strings.Contains(out, `class="pin"`) {
		t.Error("pinned notice not rendered expanded")
	}
}

func TestDefaultTemplateNoNoticesNoNodes(t *testing.T) {
	tpl, err := loadTemplate("")
	if err != nil {
		t.Fatalf("loadTemplate: %v", err)
	}
	// Unlimited traffic, no expiry, no nodes, no notices: must still render.
	vm := ViewModel{
		Username:        "u",
		TrafficUsedH:    "1.00 GiB",
		TrafficLimitH:   "∞",
		IsUnlimited:     true,
		SubURL:          "https://s/api/v1/client/sub/t",
		PageTitle:       "3m-ui 订阅",
		ClashDeepLink:   template.URL("clash://install-config?url=https%3A%2F%2Fs%2Fx"),
		SingboxDeepLink: template.URL("sing-box://import-remote?url=https%3A%2F%2Fs%2Fx"),
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, vm); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "class=\"notice\"") {
		t.Error("notice block rendered without notices")
	}
	if strings.Contains(out, "class=\"nodes\"") {
		t.Error("nodes block rendered without nodes")
	}
	if !strings.Contains(out, "不限量") {
		t.Error("unlimited label missing")
	}
	if !strings.Contains(out, "永久有效") {
		t.Error("never-expire label missing")
	}
	if !strings.Contains(out, "clash://install-config?url=") {
		t.Error("clash deep link missing")
	}
}
