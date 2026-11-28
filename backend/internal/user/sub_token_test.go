package user

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDBSub(t *testing.T) *gorm.DB {
	t.Helper()
	security.InitCredentialKey("0123456789abcdef0123456789abcdef")
	db, err := gorm.Open(sqlite.Open("file:subtoken?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ProxyUser{}, &models.Listener{}, &models.ListenerUser{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEnsureSubTokenIdempotent(t *testing.T) {
	db := testDBSub(t)
	svc := NewService(db)
	u, err := svc.Create(CreateInput{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if u.SubToken == "" {
		t.Fatal("create should set SubToken")
	}
	tok1, err := svc.EnsureSubToken(u.ID)
	if err != nil || tok1 == "" {
		t.Fatalf("tok1=%q err=%v", tok1, err)
	}
	if tok1 != u.SubToken {
		t.Fatalf("Ensure should return existing: %q vs %q", tok1, u.SubToken)
	}
	tok2, err := svc.EnsureSubToken(u.ID)
	if err != nil || tok2 != tok1 {
		t.Fatalf("expected same token, got %q vs %q", tok1, tok2)
	}
	tok3, err := svc.RotateSubToken(u.ID)
	if err != nil || tok3 == "" || tok3 == tok1 {
		t.Fatalf("rotate should change token: %q -> %q err=%v", tok1, tok3, err)
	}
}
