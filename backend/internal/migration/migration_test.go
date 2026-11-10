package migration

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newSourceDB creates a SQLite file with the given DDL/DML, matching the
// column names of the source panels' GORM models.
func newSourceDB(t *testing.T, statements []string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open(database.SQLiteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sqlStr quotes a value as a SQLite single-quoted string literal.
func sqlStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// ---------- classic 3x-ui (v2) ----------

func xuiV2DB(t *testing.T) []byte {
	settings := mustJSON(t, map[string]interface{}{
		"clients": []map[string]interface{}{
			{
				"id": "b831381d-6324-4d53-ad4f-8cda48b30811", "email": "alice",
				"flow": "xtls-rprx-vision", "totalGB": 30, "limitIp": 2,
				"expiryTime": int64(1893456000000), "enable": true,
				"subId": "sub-alice", "tgId": int64(12345), "comment": "phone",
			},
			{
				"id": "c742492e-7435-4e64-be5f-9deb59c41922", "email": "bob",
				"totalGB": 0, "enable": false, "subId": "sub-bob",
			},
		},
		"decryption": "none",
	})
	stream := mustJSON(t, map[string]interface{}{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]interface{}{
			"dest": "www.microsoft.com:443", "privateKey": "REALITY-PRIVATE-KEY",
			"shortIds": []string{"0123456789abcdef"}, "serverNames": []string{"www.microsoft.com"},
		},
	})
	wsStream := mustJSON(t, map[string]interface{}{
		"network": "ws", "security": "none",
		"wsSettings": map[string]interface{}{"path": "/ws-path-1", "headers": map[string]interface{}{"Host": "cdn.example.com"}},
	})
	clientTraffics := []struct {
		email    string
		up, down int64
	}{
		{"alice", 1024, 2048}, {"bob", 10, 20},
	}
	stmts := []string{
		`CREATE TABLE inbounds (
			id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, up INTEGER, down INTEGER,
			total INTEGER, remark TEXT, enable INTEGER, expiry_time INTEGER, listen TEXT,
			port INTEGER, protocol TEXT, settings TEXT, stream_settings TEXT, tag TEXT, sniffing TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag,listen)
			VALUES (1,'VLESS-Reality-443',1,443,'vless',%s,%s,'in-443','')`, sqlStr(settings), sqlStr(stream)),
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag,listen)
			VALUES (2,'VMess-WS-80',1,80,'vmess',%s,%s,'in-80','0.0.0.0')`, sqlStr(settings), sqlStr(wsStream)),
		`CREATE TABLE client_traffics (id INTEGER PRIMARY KEY AUTOINCREMENT, inbound_id INTEGER,
			enable INTEGER, email TEXT UNIQUE, up INTEGER, down INTEGER, all_time INTEGER,
			expiry_time INTEGER, total INTEGER, reset INTEGER, last_online INTEGER)`,
	}
	for i, c := range clientTraffics {
		stmts = append(stmts, fmt.Sprintf(
			`INSERT INTO client_traffics (id,inbound_id,enable,email,up,down,expiry_time,total) VALUES (%d,1,1,%s,%d,%d,0,0)`,
			i+1, sqlStr(c.email), c.up, c.down))
	}
	// An unsupported protocol must be skipped, not imported.
	stmts = append(stmts, fmt.Sprintf(
		`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (3,'WireGuard-51820',1,51820,'wireguard','{}','{}','in-wg')`))
	return newSourceDB(t, stmts)
}

func TestParseXUI_V2(t *testing.T) {
	snap, err := Parse(bytes.NewReader(xuiV2DB(t)))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceXUI {
		t.Fatalf("source = %q, want x-ui", snap.Source)
	}
	// The client email is a global subscriber identity in 3x-ui. alice and bob
	// each appear on BOTH inbounds, so they must become 2 users (not 4) bound
	// to 2 nodes each. Regression guard: duplicating them used to regenerate
	// UUIDs and leave the original credentials as orphaned, unbound accounts.
	if len(snap.Users) != 2 {
		t.Fatalf("users = %d, want 2 (one per distinct email)", len(snap.Users))
	}
	if len(snap.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(snap.Nodes))
	}
	if len(snap.Skipped) != 1 {
		t.Fatalf("skipped = %d, want 1 (wireguard)", len(snap.Skipped))
	}
	if len(snap.Bindings) != 4 {
		t.Fatalf("bindings = %d, want 4 (2 users x 2 nodes)", len(snap.Bindings))
	}
	// Every user must be bound to both nodes with its original credential.
	seen := map[string]int{}
	for _, b := range snap.Bindings {
		seen[b.UserRef]++
	}
	for _, u := range snap.Users {
		if seen[u.Ref] != 2 {
			t.Fatalf("user %q bound to %d nodes, want 2", u.Username, seen[u.Ref])
		}
		if u.UUID == "" {
			t.Fatalf("user %q lost its UUID", u.Username)
		}
	}
	var alice *User
	for i := range snap.Users {
		if snap.Users[i].Username == "alice" {
			alice = &snap.Users[i]
		}
	}
	if alice == nil {
		t.Fatal("alice missing")
	}
	if alice.UUID != "b831381d-6324-4d53-ad4f-8cda48b30811" {
		t.Fatalf("alice uuid = %q", alice.UUID)
	}
	if alice.TrafficLimit != gibToBytes(30) {
		t.Fatalf("alice limit = %d, want %d", alice.TrafficLimit, gibToBytes(30))
	}
	if alice.IPLimit != 2 || alice.TelegramID != 12345 || alice.Remark != "phone" {
		t.Fatalf("alice fields wrong: %+v", alice)
	}
	if alice.ExpireAt == nil || alice.ExpireAt.Year() != 2030 {
		t.Fatalf("alice expire = %v", alice.ExpireAt)
	}
	if alice.UploadBytes != 1024 || alice.DownloadBytes != 2048 {
		t.Fatalf("alice traffic = %d/%d", alice.UploadBytes, alice.DownloadBytes)
	}
	reality := snap.Nodes[0]
	if reality.Protocol != "vless" || reality.Port != "443" {
		t.Fatalf("node0 = %+v", reality)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(reality.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["reality-config"]; !ok {
		t.Fatalf("reality-config missing: %s", reality.Config)
	}
	rc := cfg["reality-config"].(map[string]interface{})
	if rc["private-key"] != "REALITY-PRIVATE-KEY" || rc["dest"] != "www.microsoft.com:443" {
		t.Fatalf("reality values wrong: %#v", rc)
	}
	if cfg["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow missing: %s", reality.Config)
	}
	if _, ok := cfg["users"]; ok {
		t.Fatal("config must not contain users")
	}
	ws := snap.Nodes[1]
	var wsCfg map[string]interface{}
	if err := json.Unmarshal([]byte(ws.Config), &wsCfg); err != nil {
		t.Fatal(err)
	}
	if wsCfg["ws-path"] != "/ws-path-1" {
		t.Fatalf("ws-path missing: %s", ws.Config)
	}
}

// A single subscriber often appears on both a uuid-based (VLESS/VMess) and a
// password-based (Trojan/SS) inbound. Both credentials must survive on the one
// canonical user, and conflicting credentials must be reported, not silently
// dropped. This is the regression guard for the duplicate-email bug.
func TestParseXUI_V2_MergesCredentialsAcrossInbounds(t *testing.T) {
	vless := mustJSON(t, map[string]interface{}{
		"clients": []map[string]interface{}{
			{"id": "11111111-1111-4111-8111-111111111111", "email": "shared", "enable": true, "totalGB": 5},
		},
	})
	trojan := mustJSON(t, map[string]interface{}{
		"clients": []map[string]interface{}{
			{"password": "trojan-pw", "email": "shared", "enable": true, "limitIp": 3},
		},
	})
	plain := mustJSON(t, map[string]interface{}{"network": "tcp", "security": "none"})
	raw := newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, remark TEXT,
			enable INTEGER, port INTEGER, protocol TEXT, settings TEXT, stream_settings TEXT, tag TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (1,'vless-in',1,443,'vless',%s,%s,'in-1')`, sqlStr(vless), sqlStr(plain)),
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (2,'trojan-in',1,8443,'trojan',%s,%s,'in-2')`, sqlStr(trojan), sqlStr(plain)),
	})
	snap, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Users) != 1 {
		t.Fatalf("users = %d, want 1 merged subscriber", len(snap.Users))
	}
	u := snap.Users[0]
	if u.UUID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("merged UUID lost: %q", u.UUID)
	}
	if u.Password != "trojan-pw" {
		t.Fatalf("merged password lost: %q", u.Password)
	}
	if u.TrafficLimit != gibToBytes(5) || u.IPLimit != 3 {
		t.Fatalf("non-credential fields not merged: %+v", u)
	}
	if len(snap.Bindings) != 2 {
		t.Fatalf("bindings = %d, want 2 (one per inbound)", len(snap.Bindings))
	}
}

// Conflicting credentials for one email are an unavoidable lossy case; the
// import must keep the first and surface a warning for operator review.
func TestParseXUI_V2_ReportsCredentialConflict(t *testing.T) {
	first := mustJSON(t, map[string]interface{}{
		"clients": []map[string]interface{}{{"id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "email": "dup", "enable": true}},
	})
	second := mustJSON(t, map[string]interface{}{
		"clients": []map[string]interface{}{{"id": "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "email": "dup", "enable": true}},
	})
	plain := mustJSON(t, map[string]interface{}{"network": "tcp", "security": "none"})
	raw := newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, remark TEXT,
			enable INTEGER, port INTEGER, protocol TEXT, settings TEXT, stream_settings TEXT, tag TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (1,'vless-1',1,443,'vless',%s,%s,'in-1')`, sqlStr(first), sqlStr(plain)),
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (2,'vless-2',1,8443,'vless',%s,%s,'in-2')`, sqlStr(second), sqlStr(plain)),
	})
	snap, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Users) != 1 || snap.Users[0].UUID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("must keep first credential: %+v", snap.Users)
	}
	found := false
	for _, w := range snap.Warnings {
		if strings.Contains(w, "conflicting UUIDs") || strings.Contains(w, "different credentials") {
			found = true
		}
	}
	if !found {
		t.Fatalf("credential conflict not reported: %v", snap.Warnings)
	}
}

// ---------- split-layout 3x-ui (v3) ----------

func xuiV3DB(t *testing.T) []byte {
	return newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, remark TEXT,
			enable INTEGER, port INTEGER, protocol TEXT, settings TEXT, stream_settings TEXT, tag TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,remark,enable,port,protocol,settings,stream_settings,tag)
			VALUES (1,'Trojan-TLS',1,8443,'trojan','{}',%s,'inbound-1')`,
			sqlStr(mustJSON(t, map[string]interface{}{"network": "tcp", "security": "tls"}))),
		`CREATE TABLE client_records (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT,
			uuid TEXT, password TEXT, flow TEXT, limit_ip INTEGER, total_gb INTEGER,
			expiry_time INTEGER, enable INTEGER, tg_id INTEGER, group_name TEXT,
			comment TEXT, sub_id TEXT)`,
		fmt.Sprintf(`INSERT INTO client_records (id,email,uuid,password,limit_ip,total_gb,expiry_time,enable,comment,sub_id)
			VALUES (1,'carol','', 'trojan-secret',3,10,0,1,'laptop','sub-carol')`),
		fmt.Sprintf(`INSERT INTO client_records (id,email,uuid,password,limit_ip,total_gb,expiry_time,enable,comment,sub_id)
			VALUES (2,'dave','', 'trojan-secret-2',0,0,0,0,'','sub-dave')`),
		`CREATE TABLE client_inbounds (client_id INTEGER, inbound_id INTEGER)`,
		`INSERT INTO client_inbounds (client_id,inbound_id) VALUES (1,1),(2,1)`,
	})
}

func TestParseXUI_V3Split(t *testing.T) {
	snap, err := Parse(bytes.NewReader(xuiV3DB(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Users) != 2 || len(snap.Nodes) != 1 || len(snap.Bindings) != 2 {
		t.Fatalf("users/nodes/bindings = %d/%d/%d", len(snap.Users), len(snap.Nodes), len(snap.Bindings))
	}
	if snap.Users[0].Password != "trojan-secret" {
		t.Fatalf("password = %q", snap.Users[0].Password)
	}
	if snap.Users[0].TrafficLimit != gibToBytes(10) || snap.Users[0].IPLimit != 3 {
		t.Fatalf("limit/ip = %d/%d", snap.Users[0].TrafficLimit, snap.Users[0].IPLimit)
	}
	if !snap.Users[0].Enabled || snap.Users[1].Enabled {
		t.Fatalf("enabled flags wrong: %v/%v", snap.Users[0].Enabled, snap.Users[1].Enabled)
	}
	if len(snap.Nodes[0].Notes) == 0 {
		t.Fatal("TLS path caveat should be reported")
	}
}

// ---------- s-ui ----------

func suiDB(t *testing.T) []byte {
	// Shapes mirror the official s-ui on-disk format (alireza0/s-ui @ main):
	// obfs is an object {type,password}, reality.short_id is a list and the
	// handshake uses server_port. See core/inbound_defaults_test.go and
	// sub/jsonService_test.go in that repository.
	vlessOpts := mustJSON(t, map[string]interface{}{
		"listen": "::", "listen_port": 443,
		"transport": map[string]interface{}{"type": "ws", "path": "/sui-ws"},
	})
	hyOpts := mustJSON(t, map[string]interface{}{
		"listen": "0.0.0.0", "listen_port": 8443, "up_mbps": 100, "down_mbps": 200,
		"obfs": map[string]interface{}{"type": "salamander", "password": "obfs-secret"},
	})
	tlsServer := mustJSON(t, map[string]interface{}{
		"enabled": true, "server_name": "reality.example.com",
		"alpn": []string{"h3"},
		"reality": map[string]interface{}{
			"enabled": true, "private_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
			"short_id":  []string{"abcd", "ef01"},
			"handshake": map[string]interface{}{"server": "www.microsoft.com", "server_port": 443},
		},
	})
	clientCfg := mustJSON(t, map[string]interface{}{
		"vless":     map[string]interface{}{"uuid": "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", "flow": "xtls-rprx-vision", "name": "eve"},
		"hysteria2": map[string]interface{}{"password": "hy2-password", "name": "eve"},
	})
	return newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, tag TEXT,
			enable INTEGER, tls_id INTEGER, addrs TEXT, out_json TEXT, options TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options)
			VALUES (1,'vless','vless-ws',1,1,%s)`, sqlStr(vlessOpts)),
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options)
			VALUES (2,'hysteria2','hy2',1,0,%s)`, sqlStr(hyOpts)),
		`CREATE TABLE tls (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, server TEXT, client TEXT)`,
		fmt.Sprintf(`INSERT INTO tls (id,name,server) VALUES (1,'reality',%s)`, sqlStr(tlsServer)),
		`CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, enable INTEGER,
			name TEXT, config TEXT, inbounds TEXT, links TEXT, volume INTEGER, expiry INTEGER,
			down INTEGER, up INTEGER, desc TEXT, "group" TEXT, remark TEXT,
			created_at INTEGER, online_at INTEGER, delay_start INTEGER, auto_reset INTEGER,
			reset_days INTEGER, next_reset INTEGER, total_up INTEGER, total_down INTEGER)`,
		fmt.Sprintf(`INSERT INTO clients (id,enable,name,config,inbounds,volume,expiry,up,down,remark,"group")
			VALUES (1,1,'eve',%s,'[1,2]',%d,%d,%d,%d,'primary','team-a')`,
			sqlStr(clientCfg), int64(5*1024*1024*1024), int64(1893456000), int64(4096), int64(8192)),
		// Unsupported type must be skipped.
		`INSERT INTO inbounds (id,type,tag,enable,tls_id,options)
			VALUES (3,'wireguard','wg',1,0,'{}')`,
	})
}

func TestParseSUI(t *testing.T) {
	snap, err := Parse(bytes.NewReader(suiDB(t)))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceSUI {
		t.Fatalf("source = %q", snap.Source)
	}
	if len(snap.Users) != 1 || len(snap.Nodes) != 2 {
		t.Fatalf("users/nodes = %d/%d", len(snap.Users), len(snap.Nodes))
	}
	if len(snap.Bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(snap.Bindings))
	}
	if len(snap.Skipped) != 1 {
		t.Fatalf("skipped = %d, want 1", len(snap.Skipped))
	}
	eve := snap.Users[0]
	if eve.Username != "eve" || eve.UUID != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
		t.Fatalf("eve = %+v", eve)
	}
	if eve.Password != "hy2-password" {
		t.Fatalf("eve password = %q", eve.Password)
	}
	if eve.TrafficLimit != 5*1024*1024*1024 || eve.UploadBytes != 4096 || eve.DownloadBytes != 8192 {
		t.Fatalf("eve counters = %d/%d/%d", eve.TrafficLimit, eve.UploadBytes, eve.DownloadBytes)
	}
	if eve.ExpireAt == nil || eve.ExpireAt.Year() != 2030 {
		t.Fatalf("eve expire = %v", eve.ExpireAt)
	}
	if eve.Group != "team-a" || eve.Remark != "primary" {
		t.Fatalf("eve group/remark = %q/%q", eve.Group, eve.Remark)
	}
	vless := snap.Nodes[0]
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(vless.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["ws-path"] != "/sui-ws" {
		t.Fatalf("ws-path missing: %s", vless.Config)
	}
	rc, ok := cfg["reality-config"].(map[string]interface{})
	if !ok || rc["private-key"] != "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0" {
		t.Fatalf("reality missing: %s", vless.Config)
	}
	if rc["dest"] != "www.microsoft.com:443" {
		t.Fatalf("reality dest should carry server_port: %v", rc["dest"])
	}
	shortIDs, ok := rc["short-id"].([]interface{})
	if !ok || len(shortIDs) != 2 || shortIDs[0] != "abcd" || shortIDs[1] != "ef01" {
		t.Fatalf("short-id must be a clean list (not a stringified array): %v", rc["short-id"])
	}
	if vless.AccessSNI != "reality.example.com" || vless.AccessALPN != "h3" {
		t.Fatalf("sni/alpn = %q/%q", vless.AccessSNI, vless.AccessALPN)
	}
	var hyCfg map[string]interface{}
	if err := json.Unmarshal([]byte(snap.Nodes[1].Config), &hyCfg); err != nil {
		t.Fatal(err)
	}
	if hyCfg["obfs"] != "salamander" || hyCfg["obfs-password"] != "obfs-secret" || hyCfg["up"] != "100 Mbps" {
		t.Fatalf("hy2 obfs object not mapped: %s", snap.Nodes[1].Config)
	}
}

// suiMinimalDB builds a small s-ui backup: one inbound and one client with the
// given per-protocol config map. Protocols in cfg drive credential selection.
func suiMinimalDB(t *testing.T, inboundType string, options map[string]interface{}, clientCfg map[string]interface{}, boundIDs string) []byte {
	t.Helper()
	return newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, tag TEXT,
			enable INTEGER, tls_id INTEGER, addrs TEXT, out_json TEXT, options TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options) VALUES (1,%s,'n1',1,0,%s)`,
			sqlStr(inboundType), sqlStr(mustJSON(t, options))),
		`CREATE TABLE tls (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, server TEXT, client TEXT)`,
		`CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, enable INTEGER,
			name TEXT, config TEXT, inbounds TEXT, links TEXT, volume INTEGER, expiry INTEGER,
			down INTEGER, up INTEGER, desc TEXT, "group" TEXT, remark TEXT,
			created_at INTEGER, online_at INTEGER, delay_start INTEGER, auto_reset INTEGER,
			reset_days INTEGER, next_reset INTEGER, total_up INTEGER, total_down INTEGER)`,
		fmt.Sprintf(`INSERT INTO clients (id,enable,name,config,inbounds,volume,expiry,up,down)
			VALUES (1,1,'c1',%s,%s,0,0,0,0)`, sqlStr(mustJSON(t, clientCfg)), sqlStr(boundIDs)),
	})
}

// A client whose protocol sections carry DIFFERENT credentials must parse
// deterministically: parsing the same bytes many times always yields the same
// UUID/Password. The earlier map-iteration implementation was random.
func TestParseSUI_DeterministicCredentials(t *testing.T) {
	raw := suiMinimalDB(t, "vless",
		map[string]interface{}{"listen": "::", "listen_port": 10001},
		map[string]interface{}{
			"vless": map[string]interface{}{"uuid": "11111111-1111-4111-8111-111111111111"},
			"vmess": map[string]interface{}{"uuid": "22222222-2222-4222-8222-222222222222"},
		}, "[1]")
	var first struct{ UUID, Password string }
	for i := 0; i < 40; i++ {
		snap, err := Parse(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Users) != 1 {
			t.Fatalf("users = %d", len(snap.Users))
		}
		u := snap.Users[0]
		// Only the bound protocol (vless) section is used; vmess is ignored.
		if u.UUID != "11111111-1111-4111-8111-111111111111" {
			t.Fatalf("run %d: uuid = %q (bound-protocol section should win)", i, u.UUID)
		}
		if i == 0 {
			first.UUID, first.Password = u.UUID, u.Password
		} else if u.UUID != first.UUID || u.Password != first.Password {
			t.Fatalf("run %d non-deterministic: %q/%q vs %q/%q", i, u.UUID, u.Password, first.UUID, first.Password)
		}
	}
}

// When several bound protocols disagree on a credential, an explicit warning
// must be produced -- and it must NEVER embed any credential value.
func TestParseSUI_CredentialConflictWarning(t *testing.T) {
	raw := newSourceDB(t, []string{
		`CREATE TABLE inbounds (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, tag TEXT,
			enable INTEGER, tls_id INTEGER, addrs TEXT, out_json TEXT, options TEXT)`,
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options) VALUES (1,'vless','vl',1,0,%s)`,
			sqlStr(mustJSON(t, map[string]interface{}{"listen": "::", "listen_port": 10001}))),
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options) VALUES (2,'vmess','vm',1,0,%s)`,
			sqlStr(mustJSON(t, map[string]interface{}{"listen": "::", "listen_port": 10002}))),
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options) VALUES (3,'trojan','tr',1,0,%s)`,
			sqlStr(mustJSON(t, map[string]interface{}{"listen": "::", "listen_port": 10003}))),
		fmt.Sprintf(`INSERT INTO inbounds (id,type,tag,enable,tls_id,options) VALUES (4,'hysteria2','hy',1,0,%s)`,
			sqlStr(mustJSON(t, map[string]interface{}{"listen": "::", "listen_port": 10004}))),
		`CREATE TABLE tls (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, server TEXT, client TEXT)`,
		`CREATE TABLE clients (id INTEGER PRIMARY KEY AUTOINCREMENT, enable INTEGER,
			name TEXT, config TEXT, inbounds TEXT, links TEXT, volume INTEGER, expiry INTEGER,
			down INTEGER, up INTEGER, desc TEXT, "group" TEXT, remark TEXT,
			created_at INTEGER, online_at INTEGER, delay_start INTEGER, auto_reset INTEGER,
			reset_days INTEGER, next_reset INTEGER, total_up INTEGER, total_down INTEGER)`,
		fmt.Sprintf(`INSERT INTO clients (id,enable,name,config,inbounds,volume,expiry,up,down)
			VALUES (1,1,'frank',%s,'[1,2,3,4]',0,0,0,0)`,
			sqlStr(mustJSON(t, map[string]interface{}{
				"vless":     map[string]interface{}{"uuid": "u-secret-VLESS"},
				"vmess":     map[string]interface{}{"uuid": "u-secret-VMESS"},
				"trojan":    map[string]interface{}{"password": "p-secret-TROJAN"},
				"hysteria2": map[string]interface{}{"password": "p-secret-HY2"},
			}))),
	})
	snap, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(snap.Warnings, "\n")
	if !strings.Contains(joined, "frank") || !strings.Contains(joined, "different UUIDs") || !strings.Contains(joined, "different passwords") {
		t.Fatalf("missing conflict warning: %v", snap.Warnings)
	}
	for _, secret := range []string{"u-secret-VLESS", "u-secret-VMESS", "p-secret-TROJAN", "p-secret-HY2"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("warning leaked credential value %q: %s", secret, joined)
		}
	}
}

// The official config key for 2022-blake3 shadowsocks is "shadowsocks16"
// (util/shadowsocks.go). Its password must be preserved, not dropped.
func TestParseSUI_Shadowsocks16Password(t *testing.T) {
	raw := suiMinimalDB(t, "shadowsocks",
		map[string]interface{}{"listen": "::", "listen_port": 10005, "method": "2022-blake3-aes-128-gcm"},
		map[string]interface{}{
			"shadowsocks16": map[string]interface{}{"password": "ss2022-secret"},
		}, "[1]")
	snap, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Users) != 1 || snap.Users[0].Password != "ss2022-secret" {
		t.Fatalf("shadowsocks16 password lost: %+v", snap.Users)
	}
	var cfg map[string]interface{}
	_ = json.Unmarshal([]byte(snap.Nodes[0].Config), &cfg)
	if cfg["cipher"] != "2022-blake3-aes-128-gcm" {
		t.Fatalf("cipher missing: %s", snap.Nodes[0].Config)
	}
}

// ---------- containers & guards ----------

func TestParseZipAndSelfGuard(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("backup/x-ui.db")
	_, _ = f.Write(xuiV2DB(t))
	zw.Close()
	snap, err := Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceXUI {
		t.Fatalf("zip parse source = %q", snap.Source)
	}
}

func TestDetectSelf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "self.db")
	g, err := testGorm(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.AutoMigrate(&models.Listener{}, &models.ProxyUser{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	src, err := DetectBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceSelf {
		t.Fatalf("detect = %q, want self", src)
	}
	if _, err := Parse(bytes.NewReader(raw)); err == nil {
		t.Fatal("Parse must reject a self backup")
	}
}

func TestDetectRejectsJunk(t *testing.T) {
	if _, err := Parse(bytes.NewReader([]byte("not a database"))); err == nil {
		t.Fatal("junk must be rejected")
	}
}

// ---------- import ----------

func testGorm(path string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open("file:"+filepath.ToSlash(path)), &gorm.Config{})
}

func TestImportAppendRenameAndDisabled(t *testing.T) {
	security.InitCredentialKey("test-secret")
	path := filepath.Join(t.TempDir(), "panel.db")
	g, err := testGorm(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.AutoMigrate(&models.Listener{}, &models.ProxyUser{}, &models.ListenerUser{}); err != nil {
		t.Fatal(err)
	}
	// Pre-existing rows must survive untouched.
	existing := models.ProxyUser{Username: "alice", UUID: "existing-uuid", SubToken: "existing-token"}
	if err := g.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	existingListener := models.Listener{Name: "VLESS-Reality-443", Protocol: "vless", Port: "9999", Enabled: true}
	if err := g.Create(&existingListener).Error; err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(bytes.NewReader(xuiV2DB(t)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Import(g, snap, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// alice/bob are each a single subscriber bound to both imported nodes.
	// Only the username collision with the pre-existing "alice" is renamed;
	// UUIDs must stay untouched (no self-collision from duplicated users).
	if res.UsersCreated != 2 || res.NodesCreated != 2 || res.BindingsCreated != 4 {
		t.Fatalf("result = %+v", res)
	}
	// One username collision (pre-existing "alice") and one listener-name
	// collision (pre-existing "VLESS-Reality-443"). Crucially, no credential
	// UUID may be regenerated: the source UUIDs must survive the import.
	if len(res.Renamed) != 2 {
		t.Fatalf("expected 2 collision renames (user + listener), got %v", res.Renamed)
	}
	for _, r := range res.Renamed {
		if strings.Contains(r, "UUID regenerated") {
			t.Fatalf("source credential was regenerated: %v", res.Renamed)
		}
	}

	var users []models.ProxyUser
	if err := g.Order("id").Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 {
		t.Fatalf("users = %d, want 3 (1 existing + 2 imported)", len(users))
	}
	if users[0].Username != "alice" || users[0].UUID != "existing-uuid" {
		t.Fatalf("existing user was modified: %+v", users[0])
	}
	var listeners []models.Listener
	if err := g.Order("id").Find(&listeners).Error; err != nil {
		t.Fatal(err)
	}
	if len(listeners) != 3 {
		t.Fatalf("listeners = %d, want 3", len(listeners))
	}
	if !listeners[0].Enabled {
		t.Fatal("existing listener was disabled")
	}
	for _, l := range listeners[1:] {
		if l.Enabled {
			t.Fatalf("imported listener %q must be disabled", l.Name)
		}
		if l.Status != "inactive" {
			t.Fatalf("imported listener %q status = %q", l.Name, l.Status)
		}
	}
	var links int64
	if err := g.Model(&models.ListenerUser{}).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if links != 4 {
		t.Fatalf("bindings = %d, want 4", links)
	}
	// Credentials must be decryptable so subscribers keep working.
	var imported models.ProxyUser
	if err := g.Where("username LIKE ?", "alice%").Order("id desc").First(&imported).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := security.Decrypt(imported.PasswordEncrypted); err != nil {
		t.Fatalf("password not decryptable: %v", err)
	}
}

func TestImportBackupFailureAborts(t *testing.T) {
	security.InitCredentialKey("test-secret")
	path := filepath.Join(t.TempDir(), "panel.db")
	g, err := testGorm(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.AutoMigrate(&models.Listener{}, &models.ProxyUser{}, &models.ListenerUser{}); err != nil {
		t.Fatal(err)
	}
	snap, err := Parse(bytes.NewReader(xuiV2DB(t)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Import(g, snap, Options{Backup: func() (string, error) {
		return "", fmt.Errorf("disk full")
	}})
	if err == nil {
		t.Fatal("backup failure must abort import")
	}
	var count int64
	g.Model(&models.ProxyUser{}).Count(&count)
	if count != 0 {
		t.Fatalf("users written despite backup failure: %d", count)
	}
}

func TestUniqueNameAndUnits(t *testing.T) {
	if gibToBytes(2) != 2*1024*1024*1024 {
		t.Fatal("GiB conversion wrong")
	}
	if !looksLikeUUID("b831381d-6324-4d53-ad4f-8cda48b30811") {
		t.Fatal("uuid detection wrong")
	}
	taken := map[string]struct{}{"a": {}, "a_imported": {}}
	n, changed := uniqueName("a", taken, "_imported")
	if n != "a_imported2" || !changed {
		t.Fatalf("uniqueName = %q/%v", n, changed)
	}
}
