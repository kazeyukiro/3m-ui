package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// officialDocExample is the rule list published verbatim in the mihomo
// docs (wiki.metacubex.one/config/rules, the config block at the top of
// the page). Every documented rule type appears here, so this test is the
// guard rail against inventing or rejecting documented syntax.
const officialDocExample = `DOMAIN,ad.com,REJECT
DOMAIN-SUFFIX,google.com,auto
DOMAIN-KEYWORD,google,auto
DOMAIN-WILDCARD,*.google.com,auto
DOMAIN-REGEX,^abc.*com,PROXY
GEOSITE,youtube,PROXY
IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
IP-CIDR6,2620:0:2d0:200::7/32,auto
IP-SUFFIX,8.8.8.8/24,PROXY
IP-ASN,13335,DIRECT
GEOIP,CN,DIRECT
SRC-GEOIP,cn,DIRECT
SRC-IP-ASN,9808,DIRECT
SRC-IP-CIDR,192.168.1.201/32,DIRECT
SRC-IP-SUFFIX,192.168.1.201/8,DIRECT
DST-PORT,80,DIRECT
SRC-PORT,7777,DIRECT
IN-PORT,7890,PROXY
IN-TYPE,SOCKS/HTTP,PROXY
IN-USER,mihomo,PROXY
IN-NAME,ss,PROXY
REMATCH-NAME,rematch1,PROXY
PROCESS-PATH,/usr/bin/wget,PROXY
PROCESS-PATH,C:\Program Files\Google\Chrome\Application\chrome.exe,PROXY
PROCESS-PATH-WILDCARD,/usr/*/wget,PROXY
PROCESS-PATH-REGEX,.*bin/wget,PROXY
PROCESS-PATH-REGEX,(?i).*Application\\chrome.*,PROXY
PROCESS-NAME,curl,PROXY
PROCESS-NAME,chrome.exe,PROXY
PROCESS-NAME,com.termux,PROXY
PROCESS-NAME-WILDCARD,*telegram*,PROXY
PROCESS-NAME-REGEX,curl$,PROXY
PROCESS-NAME-REGEX,(?i)Telegram,PROXY
PROCESS-NAME-REGEX,.*telegram.*,PROXY
UID,1001,DIRECT
NETWORK,udp,DIRECT
DSCP,4,DIRECT
RULE-SET,providername,proxy
AND,((DOMAIN,baidu.com),(NETWORK,UDP)),DIRECT
OR,((NETWORK,UDP),(DOMAIN,baidu.com)),REJECT
NOT,((DOMAIN,baidu.com)),PROXY
SUB-RULE,(NETWORK,tcp),sub-rule
MATCH,auto`

func TestParseRulesTextOfficialDocExample(t *testing.T) {
	res := ParseRulesText(officialDocExample)
	if len(res.Issues) != 0 {
		t.Fatalf("official doc example must parse cleanly, got issues: %+v", res.Issues)
	}
	if len(res.Rules) != strings.Count(officialDocExample, "\n")+1 {
		t.Fatalf("expected %d rules, got %d", strings.Count(officialDocExample, "\n")+1, len(res.Rules))
	}
	// Normalizations that must have happened.
	want := map[string]bool{
		"DOMAIN-SUFFIX,google.com,auto":         true,
		"GEOIP,cn,DIRECT":                       true, // CN lowercased like mihomo
		"NETWORK,UDP,DIRECT":                    true, // udp uppercased like mihomo
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve": true,
	}
	for _, r := range res.Rules {
		delete(want, r)
	}
	if len(want) != 0 {
		for k := range want {
			t.Errorf("expected normalized rule %q in output", k)
		}
	}
}

func TestParseRulesTextValid(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare ipv4 gains /32", "IP-CIDR,1.1.1.1,DIRECT", "IP-CIDR,1.1.1.1/32,DIRECT"},
		{"bare ipv6 gains /128", "IP-CIDR,2001:db8::1,DIRECT", "IP-CIDR,2001:db8::1/128,DIRECT"},
		{"cidr canonicalized", "IP-CIDR,10.0.0.0/8,PROXY", "IP-CIDR,10.0.0.0/8,PROXY"},
		{"domain lowercased", "domain-suffix,Example.COM,DIRECT", "DOMAIN-SUFFIX,example.com,DIRECT"},
		{"type case-insensitive", "Domain-Suffix,example.com,PROXY", "DOMAIN-SUFFIX,example.com,PROXY"},
		{"whitespace trimmed", "  DOMAIN-SUFFIX , example.com , DIRECT  ", "DOMAIN-SUFFIX,example.com,DIRECT"},
		{"multi port slash", "DST-PORT,80/443,DIRECT", "DST-PORT,80/443,DIRECT"},
		{"port range", "DST-PORT,8000-9000,PROXY", "DST-PORT,8000-9000,PROXY"},
		{"port range swapped", "DST-PORT,9000-8000,PROXY", "DST-PORT,8000-9000,PROXY"},
		{"params lowercased", "GEOIP,CN,DIRECT,NO-RESOLVE", "GEOIP,cn,DIRECT,no-resolve"},
		{"src param", "IP-CIDR,10.0.0.0/8,DIRECT,src", "IP-CIDR,10.0.0.0/8,DIRECT,src"},
		{"src implies no-resolve in mihomo; kept as src", "GEOIP,CN,DIRECT,src", "GEOIP,cn,DIRECT,src"},
		{"rule-set params allowed", "RULE-SET,gfw,PROXY,no-resolve", "RULE-SET,gfw,PROXY,no-resolve"},
		{"match", "MATCH,PROXY", "MATCH,PROXY"},
		{"match lowercase", "match,proxy", "MATCH,proxy"},
		{"in-type upper", "IN-TYPE,socks/http,PROXY", "IN-TYPE,SOCKS/HTTP,PROXY"},
		{"regex lookahead is valid regexp2", `DOMAIN-REGEX,^abc(?=.*com).*,PROXY`, `DOMAIN-REGEX,^abc(?=.*com).*,PROXY`},
		{"nested logic", "AND,((DOMAIN,a.com),(OR,((NETWORK,UDP),(DOMAIN,b.com)))),DIRECT", "AND,((DOMAIN,a.com),(OR,((NETWORK,UDP),(DOMAIN,b.com)))),DIRECT"},
		{"logic child with param", "AND,((GEOIP,CN,no-resolve),(NETWORK,TCP)),DIRECT", "AND,((GEOIP,CN,no-resolve),(NETWORK,TCP)),DIRECT"},
		{"dscp star matches all in mihomo", "DSCP,*,DIRECT", "DSCP,*,DIRECT"},
		{"dscp range", "DSCP,0-63,DIRECT", "DSCP,0-63,DIRECT"},
		{"ip-asn", "IP-ASN,13335,DIRECT", "IP-ASN,13335,DIRECT"},
		{"process path windows", `PROCESS-PATH,C:\Program Files\App\app.exe,PROXY`, `PROCESS-PATH,C:\Program Files\App\app.exe,PROXY`},
		{"in-user list", "IN-USER,mihomo/root,PROXY", "IN-USER,mihomo/root,PROXY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := ParseRulesText(tc.in)
			if len(res.Issues) != 0 {
				t.Fatalf("unexpected issues: %+v", res.Issues)
			}
			if len(res.Rules) != 1 || res.Rules[0] != tc.want {
				t.Fatalf("got %q, want %q", res.Rules, tc.want)
			}
		})
	}
}

func TestParseRulesTextInvalid(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantSub string // substring the issue reason must contain
	}{
		{"unknown type", "FOO,bar,DIRECT", "unknown rule type"},
		{"missing target", "DOMAIN-SUFFIX,example.com", "missing target"},
		{"missing payload", "DOMAIN-SUFFIX,,DIRECT", "empty payload"},
		{"match with payload", "MATCH,DIRECT,no-resolve", "MATCH takes no payload"},
		{"params on domain rule", "DOMAIN-SUFFIX,example.com,DIRECT,no-resolve", "does not take no-resolve/src"},
		{"multi port with comma", "DST-PORT,80,443,DIRECT", "must use '/'"},
		{"port star", "DST-PORT,*,DIRECT", "cannot use '*'"},
		{"port out of range", "DST-PORT,70000,DIRECT", "invalid value"},
		{"port too many ranges", "DST-PORT,1/2/3/4/5/6/7/8/9/10/11/12/13/14/15/16/17/18/19/20/21/22/23/24/25/26/27/28/29,DIRECT", "maximum 28"},
		{"uid star", "UID,*,DIRECT", "UID cannot be '*'"},
		{"uid over uint32", "UID,4294967296,DIRECT", "invalid value"},
		{"dscp over 63", "DSCP,64,DIRECT", "invalid value"},
		{"network bogus", "NETWORK,icmp,DIRECT", "only TCP/UDP"},
		{"bad cidr", "IP-CIDR,10.0.0.0/33,DIRECT", "invalid IP/CIDR"},
		{"ip suffix needs prefix", "IP-SUFFIX,8.8.8.8,PROXY", "invalid IP suffix"},
		{"bad asn", "IP-ASN,abc,DIRECT", "invalid ASN"},
		{"bad in-type", "IN-TYPE,BOGUS,PROXY", "invalid inbound type"},
		{"empty in-user entry", "IN-USER,mihomo/,PROXY", "cannot be empty"},
		{"bad regex", "DOMAIN-REGEX,[unclosed,PROXY", "invalid regex"},
		{"logic missing outer parens", "AND,(DOMAIN,a.com),DIRECT", "at least one child"},
		{"logic unbalanced", "AND,((DOMAIN,a.com),DIRECT", "missing ')'"},
		{"logic empty", "AND,(),DIRECT", "at least one child"},
		{"logic match nested", "AND,((MATCH,DIRECT),(NETWORK,UDP)),DIRECT", "cannot be nested"},
		{"logic sub-rule nested", "AND,((SUB-RULE,x),(NETWORK,UDP)),DIRECT", "cannot be nested"},
		{"not needs one child", "NOT,((DOMAIN,a.com),(NETWORK,UDP)),DIRECT", "exactly one rule"},
		{"sub-rule needs one child", "SUB-RULE,(NETWORK,tcp),(DOMAIN,b.com),sub-rule", "exactly one rule"},
		{"logic child unknown type", "AND,((FOO,a.com),(NETWORK,UDP)),DIRECT", "unknown rule type"},
		{"logic child bad regex", "AND,((DOMAIN-REGEX,[bad),(NETWORK,UDP)),DIRECT", "invalid regex"},
		{"logic child params on domain", "AND,((DOMAIN,a.com,no-resolve),(NETWORK,UDP)),DIRECT", "does not take no-resolve/src"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := ParseRulesText(tc.in)
			if len(res.Rules) != 0 {
				t.Fatalf("expected no accepted rules, got %q", res.Rules)
			}
			if len(res.Issues) != 1 {
				t.Fatalf("expected 1 issue, got %+v", res.Issues)
			}
			if !strings.Contains(res.Issues[0].Reason, tc.wantSub) {
				t.Fatalf("issue %q does not contain %q", res.Issues[0].Reason, tc.wantSub)
			}
			if res.Issues[0].Line != 1 {
				t.Fatalf("expected line 1, got %d", res.Issues[0].Line)
			}
		})
	}
}

func TestParseRulesTextMixed(t *testing.T) {
	text := `# comment line

DOMAIN-SUFFIX,good.com,DIRECT
BOGUS,x,DIRECT
DOMAIN-SUFFIX,good.com,DIRECT
  DOMAIN-SUFFIX,ws.example.com,DIRECT  
MATCH,DIRECT,extra
GEOIP,CN,DIRECT,no-resolve`
	res := ParseRulesText(text)
	if len(res.Rules) != 3 {
		t.Fatalf("expected 3 accepted rules, got %q", res.Rules)
	}
	if !contains(res.Rules, "DOMAIN-SUFFIX,ws.example.com,DIRECT") {
		t.Fatalf("whitespace was not normalized: %q", res.Rules)
	}
	if res.Duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", res.Duplicates)
	}
	if len(res.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %+v", res.Issues)
	}
	// Issue line numbers point at the original text (1-based).
	if res.Issues[0].Line != 4 || res.Issues[1].Line != 7 {
		t.Fatalf("unexpected issue line numbers: %+v", res.Issues)
	}
}

func TestParseRulesTextCap(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxImportRules+10; i++ {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,d%d.example.com,DIRECT\n", i)
	}
	res := ParseRulesText(b.String())
	if len(res.Rules) != maxImportRules {
		t.Fatalf("expected %d rules, got %d", maxImportRules, len(res.Rules))
	}
	if res.Truncated != 10 {
		t.Fatalf("expected 10 truncated, got %d", res.Truncated)
	}
}

// TestImportResultsNeverSerializeNull guards the API contract the import
// previews rely on: an all-rejected or empty batch must still produce
// JSON arrays ([]), never null, or the UI's .length / .map() calls throw.
func TestImportResultsNeverSerializeNull(t *testing.T) {
	cases := []struct {
		name string
		raw  any
	}{
		{"rules empty", ParseRulesText("")},
		{"rules all-rejected", ParseRulesText("BOGUS,x,y\nDOMAIN\n")},
	}
	for _, c := range cases {
		raw, err := json.Marshal(c.raw)
		if err != nil {
			t.Fatalf("%s: marshal: %v", c.name, err)
		}
		var generic map[string]json.RawMessage
		if err := json.Unmarshal(raw, &generic); err != nil {
			t.Fatalf("%s: unmarshal: %v", c.name, err)
		}
		for _, key := range []string{"rules", "issues"} {
			v, ok := generic[key]
			if !ok {
				continue // field not present on this type
			}
			if string(v) != "[]" && string(v)[0] != '[' {
				t.Fatalf("%s: %s must serialize as a JSON array, got %s", c.name, key, v)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
