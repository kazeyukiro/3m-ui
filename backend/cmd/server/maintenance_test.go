package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/bootstrap"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestHealthcheckUsesActualHTTPAndTLSListeners(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(strconv.FormatBool(useTLS), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("THREE_M_UI_DATA_DIR", filepath.Join(root, "data"))
			t.Setenv("THREE_M_UI_PORT", "")
			t.Setenv("PANEL_PORT", "")
			path := filepath.Join(root, "config.yaml")
			initialized, err := bootstrap.Initialize(path)
			if err != nil {
				t.Fatal(err)
			}
			db, _ := initialized.DB.DB()
			defer db.Close()
			valid := true
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/health" {
					t.Error("unexpected probe path")
				}
				if valid {
					w.Write([]byte(`{"status":"ok"}`))
				} else {
					w.Write([]byte("unrelated service"))
				}
			})
			var server *httptest.Server
			if useTLS {
				server = httptest.NewTLSServer(handler)
			} else {
				server = httptest.NewServer(handler)
			}
			defer server.Close()
			address := strings.TrimPrefix(strings.TrimPrefix(server.URL, "https://"), "http://")
			host, portText, _ := net.SplitHostPort(address)
			port, _ := strconv.Atoi(portText)
			if useTLS {
				s := acme.DefaultSettings()
				s.Enabled, s.ListenTLS = true, address
				raw, _ := json.Marshal(s)
				if err := initialized.DB.Create(&models.PanelSetting{Key: "panel_ssl", Value: string(raw)}).Error; err != nil {
					t.Fatal(err)
				}
			} else if err := config.UpdateServerFile(path, port, host, "", false); err != nil {
				t.Fatal(err)
			}
			if err := runHealthcheck(path); err != nil {
				t.Fatal(err)
			}
			valid = false
			if err := runHealthcheck(path); err == nil {
				t.Fatal("HTTP 200 without the health payload must fail")
			}
		})
	}
}

func TestStoragePathsIncludesCertificateTargetsWithoutChangingConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("THREE_M_UI_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("3M_UI_CERT_DIR", filepath.Join(root, "listener-certs"))
	path := filepath.Join(root, "config.yaml")
	initialized, err := bootstrap.Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	db, _ := initialized.DB.DB()
	defer db.Close()
	cert := filepath.Join(root, "cert.pem")
	link := filepath.Join(root, "live.pem")
	if err := os.WriteFile(cert, []byte("certificate fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cert, link); err != nil {
		t.Fatal(err)
	}
	coreConfig := filepath.Join(root, "external-core.yaml")
	if err := os.WriteFile(coreConfig, []byte("rules: [MATCH,DIRECT]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	coreLink := initialized.Config.Mihomo.Config
	if err := os.MkdirAll(filepath.Dir(coreLink), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(coreConfig, coreLink); err != nil {
		t.Fatal(err)
	}
	listenerKey := filepath.Join(root, "listener-certs", "nested", "1.key")
	if err := os.MkdirAll(filepath.Dir(listenerKey), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cert, listenerKey); err != nil {
		t.Fatal(err)
	}
	ssl := acme.DefaultSettings()
	ssl.CertFile = link
	raw, _ := json.Marshal(ssl)
	if err := initialized.DB.Create(&models.PanelSetting{Key: "panel_ssl", Value: string(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	var output bytes.Buffer
	if err := printStoragePaths(path, &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{path, initialized.Config.Database.Path, link, cert, coreLink, coreConfig, listenerKey} {
		if !strings.Contains(output.String(), expected+"\n") {
			t.Errorf("missing storage path %s", expected)
		}
	}
	if strings.Contains(output.String(), initialized.Config.JWT.Secret) {
		t.Fatal("storage inventory exposed a secret")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("read-only storage inventory modified config")
	}
}

func TestStorageInventoryRejectsNestedDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	certificates := filepath.Join(root, "certificates")
	shared := filepath.Join(root, "shared")
	for _, dir := range []string{certificates, shared} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(certificates, "linked-directory")
	if err := os.Symlink(shared, link); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := printResolvedStoragePaths([]string{certificates}, &output)
	if err == nil || !strings.Contains(err.Error(), "nested directory symlinks") {
		t.Fatalf("expected an explicit unsupported directory symlink error, got %v", err)
	}
}
