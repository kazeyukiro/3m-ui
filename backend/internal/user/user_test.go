package user

import (
	"fmt"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
)

func TestActiveCredentialsFiltering(t *testing.T) {
	security.InitCredentialKey("test-secret")
	db, err := database.InitDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)

	l1 := models.Listener{Name: "ss", Protocol: "shadowsocks", Port: "8388", Enabled: true}
	if err := db.Create(&l1).Error; err != nil {
		t.Fatal(err)
	}

	active, err := svc.Create(CreateInput{Username: "active", Password: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().Add(-time.Hour)
	expired, err := svc.Create(CreateInput{Username: "expired", Password: "p2", ExpireTime: &expiredAt})
	if err != nil {
		t.Fatal(err)
	}
	limited, err := svc.Create(CreateInput{Username: "limited", Password: "p3", TrafficLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(limited).Update("traffic_used", int64(10)).Error; err != nil {
		t.Fatal(err)
	}

	for _, id := range []uint{active.ID, expired.ID, limited.ID} {
		if err := db.Create(&models.ListenerUser{ListenerID: l1.ID, ProxyUserID: id}).Error; err != nil {
			t.Fatal(err)
		}
	}

	creds, err := svc.ActiveCredentialsByListener()
	if err != nil {
		t.Fatal(err)
	}
	if len(creds[l1.ID]) != 1 || creds[l1.ID][0].Password != "p1" {
		t.Fatalf("expected only active user credential, got %#v", creds[l1.ID])
	}
}

func TestBindListenersIsReplacement(t *testing.T) {
	security.InitCredentialKey("test-secret")
	db, err := database.InitDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)

	u, err := svc.Create(CreateInput{Username: "bind-user", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	var listeners []models.Listener
	for i := 0; i < 3; i++ {
		l := models.Listener{Name: fmt.Sprintf("n-%d", i), Protocol: "trojan", Port: fmt.Sprintf("%d", 10000+i), Enabled: true}
		if err := db.Create(&l).Error; err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, l)
	}
	if err := svc.BindListeners(u.ID, []uint{listeners[0].ID, listeners[1].ID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.BindListeners(u.ID, []uint{listeners[2].ID}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetListeners(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != listeners[2].ID {
		t.Fatalf("expected replacement binding, got %#v", got)
	}

	if err := svc.BindListeners(u.ID, []uint{listeners[2].ID, listeners[2].ID, listeners[0].ID}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.GetListeners(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != listeners[0].ID || got[1].ID != listeners[2].ID {
		t.Fatalf("expected deduplicated replacement binding, got %#v", got)
	}
	var rows []models.ListenerUser
	if err := db.Unscoped().Where("proxy_user_id = ?", u.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected soft-deleted bindings to be reused without duplicates, got %#v", rows)
	}
}

func TestDeleteDepletedKeepsDisabled(t *testing.T) {
	security.InitCredentialKey("test-secret")
	db, err := database.InitDB(t.TempDir() + "/test-del.db")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)

	active, err := svc.Create(CreateInput{Username: "keep-active", Password: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := svc.Create(CreateInput{Username: "keep-disabled", Password: "p2", Enabled: boolPtr(false)})
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().Add(-time.Hour)
	expired, err := svc.Create(CreateInput{Username: "del-expired", Password: "p3", ExpireTime: &expiredAt})
	if err != nil {
		t.Fatal(err)
	}

	n, err := svc.DeleteDepleted()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted (expired only), got %d", n)
	}
	if _, err := svc.GetByID(active.ID); err != nil {
		t.Fatalf("active user should remain: %v", err)
	}
	if _, err := svc.GetByID(disabled.ID); err != nil {
		t.Fatalf("disabled user should remain: %v", err)
	}
	if _, err := svc.GetByID(expired.ID); err == nil {
		t.Fatal("expired user should be deleted")
	}
}

func boolPtr(v bool) *bool { return &v }
