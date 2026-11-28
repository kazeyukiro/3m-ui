package migration

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// queryRows runs a query and returns each row as a lowercased-column map with
// values kept in their raw form. Callers normalise what they need.
func queryRows(db *sql.DB, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for i := range cols {
		cols[i] = strings.ToLower(cols[i])
	}
	var out []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func str(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func integer(v interface{}) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int64:
		return x
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(strings.TrimSpace(string(x)), 10, 64)
		return n
	default:
		return 0
	}
}

func truthy(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "1" || s == "true" || s == "yes"
	case []byte:
		return truthy(string(x))
	default:
		return false
	}
}

// rawJSON returns the raw JSON bytes from a row value that may be stored as
// TEXT or BLOB. Empty/invalid values yield nil.
func rawJSON(v interface{}) []byte {
	var b []byte
	switch x := v.(type) {
	case string:
		b = []byte(x)
	case []byte:
		b = x
	default:
		return nil
	}
	b = []byte(strings.TrimSpace(string(b)))
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}

func decodeObject(v interface{}) map[string]interface{} {
	b := rawJSON(v)
	if len(b) == 0 || b[0] != '{' {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

// decodeArray accepts either a raw JSON array (string/[]byte) or an already
// decoded []interface{} / []map[string]interface{} value.
func decodeArray(v interface{}) []map[string]interface{} {
	switch x := v.(type) {
	case []map[string]interface{}:
		return x
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	b := rawJSON(v)
	if len(b) == 0 || b[0] != '[' {
		return nil
	}
	var raw []map[string]interface{}
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	return raw
}

func decodeUintArray(v interface{}) []uint {
	b := rawJSON(v)
	if len(b) == 0 || b[0] != '[' {
		return nil
	}
	var raw []interface{}
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	var out []uint
	for _, item := range raw {
		if n := integer(item); n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

func nestedString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(str(m[key]))
}

func nestedObject(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	if sub, ok := m[key].(map[string]interface{}); ok {
		return sub
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// timeFromMillis interprets an Xray-style millisecond epoch; 0 means no limit.
func timeFromMillis(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

// timeFromSeconds interprets a sing-box style second epoch; 0 means no limit.
func timeFromSeconds(sec int64) *time.Time {
	if sec <= 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

// gibToBytes converts a GiB quota (as used by Xray panels) to bytes.
func gibToBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	const unit = int64(1024) * 1024 * 1024
	if value > (1<<63-1)/unit {
		return 1<<63 - 1
	}
	return value * unit
}

func looksLikeUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func newToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// protocolMap translates Xray / sing-box inbound protocol names to the
// Mihomo listener protocol names this panel supports. The bool reports whether
// the protocol is supported at all.
func protocolMap(src string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(src)) {
	case "vless":
		return "vless", true
	case "vmess":
		return "vmess", true
	case "trojan":
		return "trojan", true
	case "shadowsocks", "ss":
		return "shadowsocks", true
	case "hysteria", "hysteria2", "hy2":
		return "hysteria2", true
	case "tuic", "tuic-v5":
		return "tuic-v5", true
	case "tuic-v4":
		return "tuic-v4", true
	case "anytls":
		return "anytls", true
	case "mieru":
		return "mieru", true
	case "sudoku":
		return "sudoku", true
	case "trusttunnel":
		return "trusttunnel", true
	case "shadowquic":
		return "shadowquic", true
	case "snell":
		return "snell", true
	case "http":
		return "http", true
	case "socks", "mixed", "socks5":
		// mixed inbounds accept both HTTP and SOCKS; only the SOCKS side maps.
		return "socks", true
	case "naive":
		return "http", true
	default:
		return strings.ToLower(strings.TrimSpace(src)), false
	}
}

// encodeConfig encodes a listener config fragment. Only the top-level "users"
// key is removed: this panel injects credentials from the binding table when
// generating core config. Other keys (including protocol-level shared secrets
// such as the shadowsocks password, and the vless flow) are legitimate schema
// fields and must survive. Schema validation happens at the API boundary to
// keep this package free of any dependency on the config generator.
func encodeConfig(cfg map[string]interface{}) (string, error) {
	if len(cfg) == 0 {
		return "", nil
	}
	delete(cfg, "users")
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// findColumn returns the first present column name among candidates.
func findColumn(row map[string]interface{}, candidates ...string) (interface{}, bool) {
	for _, c := range candidates {
		if v, ok := row[strings.ToLower(c)]; ok {
			return v, true
		}
	}
	return nil, false
}
