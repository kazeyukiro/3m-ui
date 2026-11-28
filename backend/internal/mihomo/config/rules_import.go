package config

// Strict Mihomo routing-rule batch-import parser.
//
// Every rule line is validated against the mihomo v1.19.33 rule syntax as
// documented at https://wiki.metacubex.one/config/rules and implemented in
// mihomo's rules/parser.go + rules/common/*.go. Nothing here invents rule
// types, payload shapes or params: the whitelists and per-type checks below
// mirror those sources one-to-one, so a line accepted here is a line the
// mihomo core accepts (target existence is the only thing checked later, at
// generate/apply time, because it depends on the live proxy list).

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/dlclark/regexp2"
)

// maxImportRules caps a single batch import so a pasted rule-provider dump
// cannot bloat the editor or the request.
const maxImportRules = 5000

// ruleTypes is the full mihomo v1.19.33 rule-type set (rules/parser.go,
// 37 cases; IP-CIDR6 is an alias of IP-CIDR handled by the same case).
var ruleTypes = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"DOMAIN-REGEX": true, "DOMAIN-WILDCARD": true,
	"GEOSITE": true,
	"IP-CIDR": true, "IP-CIDR6": true, "SRC-IP-CIDR": true,
	"IP-SUFFIX": true, "SRC-IP-SUFFIX": true,
	"IP-ASN": true, "SRC-IP-ASN": true,
	"GEOIP": true, "SRC-GEOIP": true,
	"DST-PORT": true, "SRC-PORT": true, "IN-PORT": true,
	"DSCP":         true,
	"PROCESS-NAME": true, "PROCESS-PATH": true,
	"PROCESS-NAME-REGEX": true, "PROCESS-PATH-REGEX": true,
	"PROCESS-NAME-WILDCARD": true, "PROCESS-PATH-WILDCARD": true,
	"NETWORK": true, "UID": true,
	"IN-TYPE": true, "IN-USER": true, "IN-NAME": true, "REMATCH-NAME": true,
	"RULE-SET": true,
	"AND":      true, "OR": true, "NOT": true, "SUB-RULE": true,
	"MATCH": true,
}

// ruleParamTypes are the types whose trailing params mihomo actually reads
// (rules/parser.go calls ParseParams only for these). Every other type
// silently ignores params, and the official docs document no-resolve / src
// only for "rules about the target IP" — so an import carrying params on
// any other type is rejected instead of silently dropping them.
var ruleParamTypes = map[string]bool{
	"GEOIP": true, "SRC-GEOIP": true,
	"IP-ASN": true, "SRC-IP-ASN": true,
	"IP-CIDR": true, "IP-CIDR6": true, "SRC-IP-CIDR": true,
	"IP-SUFFIX": true, "SRC-IP-SUFFIX": true,
	"RULE-SET": true,
}

// ruleCommaPayloadTypes may contain commas inside the payload: mihomo's
// ParseRulePayload takes the LAST field as target and joins everything
// between type and target back with commas. These types take no params.
var ruleCommaPayloadTypes = map[string]bool{
	"NOT": true, "OR": true, "AND": true, "SUB-RULE": true,
	"DOMAIN-REGEX": true, "PROCESS-NAME-REGEX": true, "PROCESS-PATH-REGEX": true,
}

// inTypeValues mirrors constant/metadata.go ParseType — the legal IN-TYPE
// values (multi-value lines are '/'-separated). "SOCKS" expands to
// SOCKS4+SOCKS5 in mihomo; it is accepted here for compatibility.
var inTypeValues = map[string]bool{
	"HTTP": true, "HTTPS": true, "SOCKS": true, "SOCKS4": true, "SOCKS5": true,
	"SHADOWSOCKS": true, "SNELL": true, "VMESS": true, "VLESS": true,
	"REDIR": true, "TPROXY": true, "TROJAN": true, "TUNNEL": true, "TUN": true,
	"TUIC": true, "HYSTERIA2": true, "ANYTLS": true, "MIERU": true,
	"SUDOKU": true, "TRUSTTUNNEL": true, "SHADOWQUIC": true, "INNER": true,
}

// RuleImportIssue is one rejected input line, with its 1-based line number
// so the UI can point at the exact row in the pasted text.
type RuleImportIssue struct {
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// RuleImportResult is the outcome of parsing a batch-import blob: the
// normalized rule lines ready to append, plus everything that was rejected
// or dropped (duplicates within the batch, lines over the cap).
type RuleImportResult struct {
	Rules      []string          `json:"rules"`
	Duplicates int               `json:"duplicates"`
	Issues     []RuleImportIssue `json:"issues"`
	Truncated  int               `json:"truncated"`
}

// ParseRulesText parses and validates a batch-import blob. Accepted lines
// are returned normalized (canonical TYPE,payload,target[,params] form,
// bare IPs gaining an explicit /32 or /128). Invalid lines never abort the
// batch: each is reported in Issues with its line number and reason.
func ParseRulesText(text string) RuleImportResult {
	// Slices start non-nil so the JSON contract is [] rather than null:
	// the import preview reads .length / iterates without null checks.
	res := RuleImportResult{Rules: []string{}, Issues: []RuleImportIssue{}}
	seen := make(map[string]bool)
	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "#") {
			continue // blank lines and full-line comments are skipped silently
		}
		norm, err := parseRuleImportLine(s)
		if err != nil {
			res.Issues = append(res.Issues, RuleImportIssue{Line: lineNo, Text: truncateForIssue(s), Reason: err.Error()})
			continue
		}
		if seen[norm] {
			res.Duplicates++
			continue
		}
		seen[norm] = true
		if len(res.Rules) >= maxImportRules {
			res.Truncated++
			continue
		}
		res.Rules = append(res.Rules, norm)
	}
	return res
}

func truncateForIssue(s string) string {
	const max = 120
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// parseRuleImportLine validates one rule line and returns its canonical
// form. The comma split mirrors mihomo's ParseRulePayload exactly.
func parseRuleImportLine(line string) (string, error) {
	items := splitRuleFields(line)
	tp := strings.ToUpper(items[0])
	if !ruleTypes[tp] {
		return "", fmt.Errorf("unknown rule type %q (see wiki.metacubex.one/config/rules)", items[0])
	}

	if tp == "MATCH" {
		// Official syntax: MATCH,target — no payload, no params. mihomo
		// silently ignores extra fields; the import rejects them because
		// they are meaningless and usually a paste mistake.
		if len(items) != 2 {
			return "", fmt.Errorf("MATCH takes no payload or params (syntax: MATCH,target)")
		}
		return "MATCH," + items[1], nil
	}

	var payload, target string
	var params []string
	if ruleCommaPayloadTypes[tp] {
		// target is the last field; payload keeps its commas (no params).
		if len(items) < 3 {
			return "", fmt.Errorf("missing target (syntax: %s,payload,target)", tp)
		}
		target = items[len(items)-1]
		payload = strings.Join(items[1:len(items)-1], ",")
		if err := validateCommaPayload(tp, payload); err != nil {
			return "", err
		}
		return tp + "," + payload + "," + target, nil
	}

	if len(items) < 3 {
		return "", fmt.Errorf("missing target (syntax: %s,payload,target)", tp)
	}
	payload, target = items[1], items[2]
	params = items[3:]
	if len(params) > 0 && !ruleParamTypes[tp] {
		if isRangeListType(tp) {
			// The classic paste mistake: DST-PORT,80,443,DIRECT — commas
			// separate fields, multi-value lists must use '/'.
			return "", fmt.Errorf("%s takes no params; multi-value lists must use '/' (e.g. %s,80/443,DIRECT)", tp, tp)
		}
		return "", fmt.Errorf("%s does not take no-resolve/src params (official docs: only target-IP rules do)", tp)
	}
	normParams := make([]string, 0, len(params))
	for _, p := range params {
		switch strings.ToLower(p) {
		case "no-resolve", "src":
			normParams = append(normParams, strings.ToLower(p))
		default:
			return "", fmt.Errorf("unknown param %q (only no-resolve / src are defined)", p)
		}
	}
	normPayload, err := validateRulePayload(tp, payload)
	if err != nil {
		return "", err
	}
	out := tp + "," + normPayload + "," + target
	if len(normParams) > 0 {
		out += "," + strings.Join(normParams, ",")
	}
	return out, nil
}

// splitRuleFields splits on commas and trims whitespace around each field.
// mihomo trims spaces only; trimming all whitespace here is a pure input
// normalization — the canonical line we emit is what gets persisted.
func splitRuleFields(line string) []string {
	parts := strings.Split(line, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		items = append(items, strings.TrimSpace(p))
	}
	return items
}

// validateCommaPayload validates the payloads of the comma-carrying types:
// the three regex types (compiled with the same regexp2 engine mihomo
// uses) and the logic rules AND/OR/NOT/SUB-RULE.
func validateCommaPayload(tp, payload string) error {
	if payload == "" {
		return fmt.Errorf("empty payload")
	}
	switch tp {
	case "DOMAIN-REGEX", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX":
		if _, err := regexp2.Compile(payload, regexp2.IgnoreCase); err != nil {
			return fmt.Errorf("invalid regex %q: %v", payload, err)
		}
		return nil
	default: // AND / OR / NOT / SUB-RULE
		return validateLogicPayload(tp, payload)
	}
}

// validateLogicPayload mirrors rules/logic/logic.go: the payload must be
// parenthesized, parens must balance, children are the top-level groups,
// and MATCH / SUB-RULE may not nest. NOT and SUB-RULE take exactly one
// child; AND/OR take at least one.
func validateLogicPayload(tp, payload string) error {
	if tp == "SUB-RULE" {
		// mihomo's NewSubRule wraps the payload in parens before parsing.
		payload = "(" + payload + ")"
	}
	if !strings.HasPrefix(payload, "(") || !strings.HasSuffix(payload, ")") {
		return fmt.Errorf("logic rule payload must be wrapped in parentheses, e.g. AND,((DOMAIN,a.com),(NETWORK,UDP)),DIRECT")
	}
	children, err := logicChildren(payload)
	if err != nil {
		return err
	}
	if len(children) == 0 {
		return fmt.Errorf("logic rule needs at least one child rule")
	}
	if (tp == "NOT" || tp == "SUB-RULE") && len(children) != 1 {
		return fmt.Errorf("%s must contain exactly one rule", tp)
	}
	for _, c := range children {
		if err := validateLogicChild(c); err != nil {
			return err
		}
	}
	return nil
}

// logicChildren returns the top-level parenthesized groups of a logic
// payload, skipping the outermost full-range group.
func logicChildren(payload string) ([]string, error) {
	var stack []int
	type rng struct{ start, end int }
	var ranges []rng
	for i, c := range payload {
		switch c {
		case '(':
			stack = append(stack, i)
		case ')':
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced ')' in %s", payload)
			}
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			ranges = append(ranges, rng{start, i})
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("missing ')' in %s", payload)
	}
	// mihomo sorts the ranges by start before picking top-level groups;
	// without the sort, an inner group closing before its parent would be
	// mistaken for a sibling.
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	var top []rng
	for _, r := range ranges {
		if r.start == 0 && r.end == len(payload)-1 {
			continue // outermost group
		}
		contained := false
		for _, t := range top {
			if t.start < r.start && t.end > r.end {
				contained = true
				break
			}
		}
		if !contained {
			top = append(top, r)
		}
	}
	out := make([]string, 0, len(top))
	for _, r := range top {
		out = append(out, payload[r.start+1:r.end])
	}
	return out, nil
}

// validateLogicChild validates one nested rule line (without target —
// payloadToRule parses children with needTarget=false).
func validateLogicChild(child string) error {
	items := splitRuleFields(child)
	tp := strings.ToUpper(items[0])
	if tp == "MATCH" || tp == "SUB-RULE" {
		return fmt.Errorf("%s cannot be nested inside a logic rule", tp)
	}
	if tp == "" {
		return fmt.Errorf("empty rule type in logic child %q", child)
	}
	if !ruleTypes[tp] {
		return fmt.Errorf("unknown rule type %q in logic rule", items[0])
	}
	// Children carry no target: TYPE,payload[,params...]
	if len(items) < 2 {
		return fmt.Errorf("logic child %q is missing its payload", child)
	}
	if ruleCommaPayloadTypes[tp] {
		payload := strings.Join(items[1:], ",")
		return validateCommaPayload(tp, payload)
	}
	if _, err := validateRulePayload(tp, items[1]); err != nil {
		return err
	}
	// params are legal only on the param types, same as top-level lines.
	if len(items) > 2 && !ruleParamTypes[tp] {
		return fmt.Errorf("%s does not take no-resolve/src params (official docs: only target-IP rules do)", tp)
	}
	for _, p := range items[2:] {
		switch strings.ToLower(p) {
		case "no-resolve", "src":
		default:
			return fmt.Errorf("unknown param %q in logic rule (only no-resolve / src are defined)", p)
		}
	}
	return nil
}

// validateRulePayload mirrors the per-type payload checks of the
// rules/common constructors. Returns the canonical payload form.
func validateRulePayload(tp, payload string) (string, error) {
	if payload == "" {
		return "", fmt.Errorf("empty payload")
	}
	switch tp {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-WILDCARD",
		"PROCESS-NAME-WILDCARD", "PROCESS-PATH-WILDCARD",
		"PROCESS-NAME", "PROCESS-PATH":
		// mihomo only lowercases (DOMAIN*) / stores these verbatim; no
		// further static validation exists.
		if tp == "DOMAIN" || tp == "DOMAIN-SUFFIX" || tp == "DOMAIN-KEYWORD" || tp == "DOMAIN-WILDCARD" {
			return strings.ToLower(payload), nil
		}
		return payload, nil
	case "IN-USER", "IN-NAME", "REMATCH-NAME":
		// '/'-separated list; every entry must be non-empty (mirrors
		// NewInUser / NewInName / NewRematchName).
		parts := strings.Split(payload, "/")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				return "", fmt.Errorf("%s entries cannot be empty ('/'-separated list)", tp)
			}
			out = append(out, p)
		}
		return strings.Join(out, "/"), nil
	case "GEOSITE":
		// geodata category files are lowercase; mihomo loads them by name.
		return strings.ToLower(payload), nil
	case "GEOIP", "SRC-GEOIP":
		// "lan" is a built-in pseudo-code; other codes are resolved from
		// geodata/mmdb at runtime and cannot be checked offline.
		return strings.ToLower(payload), nil
	case "IP-ASN", "SRC-IP-ASN":
		if !isAllDigits(payload) || len(payload) > 10 {
			return "", fmt.Errorf("invalid ASN %q (expected a number, e.g. 13335)", payload)
		}
		return payload, nil
	case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR":
		// netip.ParsePrefix requires an explicit prefix length; a bare IP
		// is normalized to a host route (/32 or /128).
		p := payload
		if !strings.Contains(p, "/") {
			if strings.Contains(p, ":") {
				p += "/128"
			} else {
				p += "/32"
			}
		}
		pfx, err := netip.ParsePrefix(p)
		if err != nil {
			return "", fmt.Errorf("invalid IP/CIDR %q (expected e.g. 1.1.1.1/32)", payload)
		}
		return pfx.String(), nil
	case "IP-SUFFIX", "SRC-IP-SUFFIX":
		// Unlike IP-CIDR, mihomo does not accept a bare IP here: the
		// suffix length is the whole point of the rule.
		pfx, err := netip.ParsePrefix(payload)
		if err != nil {
			return "", fmt.Errorf("invalid IP suffix %q (expected e.g. 8.8.8.8/24)", payload)
		}
		return pfx.String(), nil
	case "DST-PORT", "SRC-PORT", "IN-PORT":
		// mihomo's NewPort rejects "*": an empty range set is errPayload.
		if payload == "*" {
			return "", fmt.Errorf("port rules cannot use '*' (list ports, e.g. 80/443 or 8000-9000)")
		}
		return validatePortPayload(payload, 65535, false)
	case "UID":
		// mihomo's NewUid rejects an empty range set, so "*" alone is
		// invalid; UID is uint32.
		if payload == "*" {
			return "", fmt.Errorf("UID cannot be '*' (expected e.g. 1001 or 1000-2000)")
		}
		return validatePortPayload(payload, 4294967295, false)
	case "DSCP":
		// DSCP keeps "*" legal (an empty range set matches every value).
		return validatePortPayload(payload, 63, true)
	case "NETWORK":
		switch strings.ToUpper(payload) {
		case "TCP", "UDP":
			return strings.ToUpper(payload), nil
		}
		return "", fmt.Errorf("invalid network %q (only TCP/UDP)", payload)
	case "IN-TYPE":
		return validateInType(payload)
	case "RULE-SET":
		// Provider name; existence is checked when the config is loaded.
		return payload, nil
	}
	return payload, nil
}

// validatePortPayload mirrors common/utils/ranges.go: '*' means all
// values (only as the whole payload, and only where mihomo accepts an
// empty range set), fields are '/'-separated (commas are field separators
// inside a rule line), each field is N or N-M, at most 28 fields, and the
// range set must be non-empty.
func validatePortPayload(payload string, max uint64, allowStar bool) (string, error) {
	if payload == "*" {
		if allowStar {
			return payload, nil
		}
		return "", fmt.Errorf("'*' is not allowed here")
	}
	fields := strings.Split(payload, "/")
	if len(fields) > 28 {
		return "", fmt.Errorf("too many ranges (maximum 28, use '/' to separate)")
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		bounds := strings.Split(f, "-")
		if len(bounds) > 2 {
			return "", fmt.Errorf("invalid range %q (expected N or N-M)", f)
		}
		nums := make([]uint64, 0, 2)
		for _, b := range bounds {
			b = strings.TrimSpace(b)
			n, err := strconv.ParseUint(b, 10, 64)
			if err != nil || n > max {
				return "", fmt.Errorf("invalid value %q (expected 0-%d)", b, max)
			}
			nums = append(nums, n)
		}
		if len(bounds) == 2 && nums[0] > nums[1] {
			nums[0], nums[1] = nums[1], nums[0] // mihomo's NewRange swaps
		}
		if len(bounds) == 2 {
			out = append(out, fmt.Sprintf("%d-%d", nums[0], nums[1]))
		} else {
			out = append(out, strconv.FormatUint(nums[0], 10))
		}
	}
	if len(out) == 0 {
		return "", fmt.Errorf("empty range (expected e.g. 80, 80/443 or 8000-9000)")
	}
	return strings.Join(out, "/"), nil
}

// validateInType mirrors constant/metadata.go ParseType: '/'-separated
// values from the inbound-type whitelist.
func validateInType(payload string) (string, error) {
	parts := strings.Split(payload, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToUpper(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		if !inTypeValues[v] {
			return "", fmt.Errorf("invalid inbound type %q (see wiki.metacubex.one/config/inbound/listeners)", p)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("empty IN-TYPE (expected e.g. SOCKS/HTTP)")
	}
	return strings.Join(out, "/"), nil
}

// isRangeListType reports whether tp carries a '/'-separated value list
// (ports / UID / DSCP), where a stray extra field almost always means the
// user separated values with commas.
func isRangeListType(tp string) bool {
	switch tp {
	case "DST-PORT", "SRC-PORT", "IN-PORT", "UID", "DSCP":
		return true
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
