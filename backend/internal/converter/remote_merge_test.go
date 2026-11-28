package converter

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRemoteMergeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RemoteServer{},
		&models.RemoteNodeMirror{},
		&models.ProxyUserRemoteNode{},
		&models.ProxyUser{},
		&models.ListenerUser{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// Disabling (or deleting) a remote must retract its nodes from subscriptions.
// The mirror row itself may survive, but it must not contribute proxies while
// its remote server is off.
func TestLoadBoundRemoteMirrorsSkipsDisabledRemote(t *testing.T) {
	db := setupRemoteMergeDB(t)

	enabledRemote := models.RemoteServer{Name: "on", BaseURL: "https://on.example.com", Enabled: true}
	disabledRemote := models.RemoteServer{Name: "off", BaseURL: "https://off.example.com"}
	if err := db.Create(&enabledRemote).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&disabledRemote).Error; err != nil {
		t.Fatal(err)
	}
	// RemoteServer.Enabled carries a DB default of true, so an explicit false
	// in the struct is coerced on insert. Force it with a column update, the
	// same way the panel's disable endpoint does.
	if err := db.Model(&disabledRemote).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	onMirror := models.RemoteNodeMirror{
		RemoteServerID: enabledRemote.ID, RemoteNodeID: 1, Name: "on-node",
		ShareURI: "trojan://on@on.example.com:443#on-node", Enabled: true,
	}
	offMirror := models.RemoteNodeMirror{
		RemoteServerID: disabledRemote.ID, RemoteNodeID: 2, Name: "off-node",
		ShareURI: "trojan://off@off.example.com:443#off-node", Enabled: true,
	}
	if err := db.Create(&onMirror).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&offMirror).Error; err != nil {
		t.Fatal(err)
	}
	user := models.ProxyUser{Username: "u1"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range []models.RemoteNodeMirror{onMirror, offMirror} {
		if err := db.Create(&models.ProxyUserRemoteNode{
			ProxyUserID: user.ID, RemoteNodeMirrorID: m.ID,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	got, err := loadBoundRemoteMirrors(db, user.ID)
	if err != nil {
		t.Fatalf("loadBoundRemoteMirrors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the enabled remote's mirror, got %d", len(got))
	}
	if got[0].Name != "on-node" {
		t.Fatalf("unexpected mirror survived: %q", got[0].Name)
	}
}

// A user holding only remote cluster mirrors (zero local listener bindings)
// must still get a subscription — the "not bound to any listener" error is
// only fatal when nothing else can supply nodes.
func TestMirrorOnlyUserGetsBase64AndSingboxSubscriptions(t *testing.T) {
	db := setupRemoteMergeDB(t)

	remote := models.RemoteServer{Name: "edge", BaseURL: "https://edge.example.com", Enabled: true}
	if err := db.Create(&remote).Error; err != nil {
		t.Fatal(err)
	}
	mirror := models.RemoteNodeMirror{
		RemoteServerID: remote.ID, RemoteNodeID: 7, Name: "edge-node-1",
		ShareURI:   "trojan://edgepass1@edge1.example.com:8443?sni=edge1.example.com#edge-node-1",
		ClientYAML: "name: edge-node-1\ntype: trojan\nserver: edge1.example.com\nport: 8443\npassword: edgepass1\nsni: edge1.example.com\n",
		Enabled:    true,
	}
	if err := db.Create(&mirror).Error; err != nil {
		t.Fatal(err)
	}
	pu := models.ProxyUser{Username: "mirror-only", Enabled: true}
	if err := db.Create(&pu).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ProxyUserRemoteNode{
		ProxyUserID: pu.ID, RemoteNodeMirrorID: mirror.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	noopGen := func(models.Listener, string, []user.Credential) ([]string, error) {
		t.Fatal("URI generator must not run: the user has no local listeners")
		return nil, nil
	}
	raw, err := GenerateUserBase64Subscription(db, pu, nil, noopGen)
	if err != nil {
		t.Fatalf("base64 subscription for mirror-only user: %v", err)
	}
	decoded, decErr := base64.StdEncoding.DecodeString(string(raw))
	if decErr != nil {
		t.Fatalf("base64 subscription is not base64: %v", decErr)
	}
	if !strings.Contains(string(decoded), "trojan://edgepass1@edge1.example.com:8443") ||
		!strings.Contains(string(decoded), "#edge-node-1") {
		t.Fatalf("base64 subscription missing mirrored node: %s", decoded)
	}

	sb, err := GenerateUserSingboxSubscription(db, pu, nil)
	if err != nil {
		t.Fatalf("sing-box subscription for mirror-only user: %v", err)
	}
	if !strings.Contains(string(sb), "R1-edge-node-1") || !strings.Contains(string(sb), "trojan") {
		t.Fatalf("sing-box subscription missing mirrored outbound: %s", sb)
	}
}
