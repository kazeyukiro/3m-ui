package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestInitializePreservesCredentialsAndConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("THREE_M_UI_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("THREE_M_UI_ADMIN_PASSWORD", "")
	t.Setenv("THREE_M_UI_ADMIN_USERNAME", "")
	t.Setenv("THREE_M_UI_PORT", "8099")
	path := filepath.Join(root, "config", "config.yaml")
	first, err := Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	firstDB, _ := first.DB.DB()
	defer firstDB.Close()
	if !first.Created || first.Username != "admin" || len(first.Password) < 24 || first.Password == "admin" {
		t.Fatal("expected a newly generated administrator")
	}
	if first.Config.Server.Port != 8099 || first.Config.JWT.Secret == first.Config.Security.CredentialKey {
		t.Fatal("configuration defaults or independent secrets are incorrect")
	}
	var user models.User
	if err := first.DB.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword || !auth.CheckPasswordHash(first.Password, user.PasswordHash) {
		t.Fatal("administrator must use a password hash and require a password change")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(first.Password)) {
		t.Fatal("password must not be written to config")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode = %o", info.Mode().Perm())
	}
	var printed bytes.Buffer
	first.PrintCredentials(&printed)
	if !bytes.Contains(printed.Bytes(), []byte(first.Password)) {
		t.Fatal("initial credentials must be available once")
	}
	firstDB.Close()
	t.Setenv("THREE_M_UI_ADMIN_PASSWORD", "replacement-must-not-apply")
	second, err := Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	secondDB, _ := second.DB.DB()
	defer secondDB.Close()
	if second.Created || second.Password != "" {
		t.Fatal("restart recreated administrator")
	}
	printed.Reset()
	second.PrintCredentials(&printed)
	if printed.Len() != 0 {
		t.Fatal("restart displayed existing credentials")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("restart changed persisted configuration")
	}
	var persisted models.User
	second.DB.First(&persisted)
	if persisted.PasswordHash != user.PasswordHash {
		t.Fatal("restart changed password hash")
	}
}

func TestInitializeRejectsBadConfigWithoutReplacingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	bad := []byte("server: [broken\n")
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(path); err == nil {
		t.Fatal("malformed existing config must fail")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(bad, after) {
		t.Fatal("invalid config was overwritten")
	}
}

func TestExplicitPasswordAndReset(t *testing.T) {
	root := t.TempDir()
	t.Setenv("THREE_M_UI_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("THREE_M_UI_ADMIN_USERNAME", "operator")
	t.Setenv("THREE_M_UI_ADMIN_PASSWORD", "explicit-bootstrap-password")
	initialized, err := Initialize(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := initialized.DB.DB()
	defer sqlDB.Close()
	if initialized.Username != "operator" || initialized.Password != "explicit-bootstrap-password" {
		t.Fatal("explicit bootstrap credentials ignored")
	}
	password, err := auth.GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ResetAdminPassword(initialized.DB, password); err != nil {
		t.Fatal(err)
	}
	var user models.User
	initialized.DB.First(&user)
	if !auth.CheckPasswordHash(password, user.PasswordHash) || user.SessionVersion != 2 || !user.MustChangePassword {
		t.Fatal("reset must invalidate sessions and require password change")
	}
	if err := auth.ResetAdminPassword(initialized.DB, ""); err == nil {
		t.Fatal("empty reset must not restore admin/admin")
	}
}
