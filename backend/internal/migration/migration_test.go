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
	if len(snap.Users) != 4 {
		t.Fatalf("users = %d, want 4 (2 per inbound)", len(snap.Users))
	}
	if len(snap.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(snap.Nodes))
	}
	if len(snap.Skipped) != 1 {
		t.Fatalf("skipped = %d, want 1 (wireguard)", len(snap.Skipped))
	}
	if len(snap.Bindings) != 4 {
		t.Fatalf("bindings = %d, want 4", len(snap.Bindings))
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
	vlessOpts := mustJSON(t, map[string]interface{}{
		"listen": "::", "listen_port": 443,
		"transport": map[string]interface{}{"type": "ws", "path": "/sui-ws"},
	})
	hyOpts := mustJSON(t, map[string]interface{}{
		"listen": "0.0.0.0", "listen_port": 8443, "up_mbps": 100, "down_mbps": 200,
		"obfs": "salamander", "obfs_password": "obfs-secret",
	})
	tlsServer := mustJSON(t, map[string]interface{}{
		"enabled": true, "server_name": "reality.example.com",
		"alpn": []string{"h3"},
		"reality": map[string]interface{}{
			"enabled": true, "private_key": "SUI-REALITY-KEY", "short_id": "abcd",
			"handshake": map[string]interface{}{"server": "www.microsoft.com", "port": 443},
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
	if !ok || rc["private-key"] != "SUI-REALITY-KEY" {
		t.Fatalf("reality missing: %s", vless.Config)
	}
	if vless.AccessSNI != "reality.example.com" || vless.AccessALPN != "h3" {
		t.Fatalf("sni/alpn = %q/%q", vless.AccessSNI, vless.AccessALPN)
	}
	var hyCfg map[string]interface{}
	if err := json.Unmarshal([]byte(snap.Nodes[1].Config), &hyCfg); err != nil {
		t.Fatal(err)
	}
	if hyCfg["obfs"] != "salamander" || hyCfg["up"] != "100 Mbps" {
		t.Fatalf("hy2 config = %s", snap.Nodes[1].Config)
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
	if res.UsersCreated != 4 || res.NodesCreated != 2 || res.BindingsCreated != 4 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Renamed) < 2 {
		t.Fatalf("expected collision renames, got %v", res.Renamed)
	}

	var users []models.ProxyUser
	if err := g.Order("id").Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 5 {
		t.Fatalf("users = %d, want 5", len(users))
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
