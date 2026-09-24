package user

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestQuickCreateAutoUsername(t *testing.T) {
	security.InitCredentialKey("test-secret-key-32bytes-long!!!!")
	db, err := gorm.Open(sqlite.Open("file:quick_user?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ProxyUser{}, &models.Listener{}, &models.ListenerUser{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}
	res, err := svc.QuickCreate(QuickCreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.User.Username == "" || !strings.HasPrefix(res.User.Username, "u") {
		t.Fatalf("username=%q", res.User.Username)
	}
	if res.Password == "" || res.UUID == "" {
		t.Fatal("missing secrets")
	}
}
