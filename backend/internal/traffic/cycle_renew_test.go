package traffic

import (
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:cycle?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ProxyUser{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestExpireRenewOnlyAfterExpiry(t *testing.T) {
	db := testDB(t)
	now := time.Now().UTC()
	// Within 24h window but not yet expired — must NOT extend.
	u := models.ProxyUser{
		Username: "soon", UUID: "u1", SubToken: "t1",
		ExpireTime: now.Add(12 * time.Hour), ExpireRenewDays: 30,
		Enabled: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	MaybeApplyUserCycles(db)
	var got models.ProxyUser
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !got.ExpireTime.Equal(u.ExpireTime) {
		t.Fatalf("pre-expiry renew mutated expire_time: %v -> %v", u.ExpireTime, got.ExpireTime)
	}

	// Already expired — must extend from now by 30 days.
	past := now.Add(-time.Hour)
	if err := db.Model(&got).Update("expire_time", past).Error; err != nil {
		t.Fatal(err)
	}
	MaybeApplyUserCycles(db)
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !got.ExpireTime.After(now.Add(29 * 24 * time.Hour)) {
		t.Fatalf("expected ~30d extension, got %v", got.ExpireTime)
	}
}

func TestTrafficCycleDoesNotWipeOnFirstEnable(t *testing.T) {
	db := testDB(t)
	u := models.ProxyUser{
		Username: "tuser", UUID: "u2", SubToken: "t2",
		TrafficUsed: 12345, TrafficResetDays: 7, Enabled: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	MaybeApplyUserCycles(db)
	var got models.ProxyUser
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.TrafficUsed != 12345 {
		t.Fatalf("first tick wiped traffic: %d", got.TrafficUsed)
	}
	if got.LastTrafficCycleReset == nil {
		t.Fatal("expected last_traffic_cycle_reset to be set")
	}
}
