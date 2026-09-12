package user

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestStartOnFirstUseIgnoresExpireUntilTouch(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	u := models.ProxyUser{Username: "fu", PasswordEncrypted: "x", UUID: "u", Enabled: true, StartOnFirstUse: true, ExpireTime: past}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if !IsCredentialActive(u) {
		t.Fatal("should be active before first use despite past expire")
	}
	TouchFirstUse(db, u.ID)
	var got models.ProxyUser
	_ = db.First(&got, u.ID)
	if got.FirstConnectedAt == nil {
		t.Fatal("expected first_connected_at")
	}
}
