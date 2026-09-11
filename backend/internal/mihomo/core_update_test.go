package mihomo

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gopkg.in/yaml.v3"
)

type coreRoundTrip func(*http.Request) (*http.Response, error)

func (f coreRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureCore(t *testing.T, body string) (*Service, *config.Config) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(AllowBinaryPathPrefixForTesting(dir))
	binary := filepath.Join(dir, "bundled")
	if err := os.WriteFile(binary, []byte(fakeCore("v1.0.0", body, false)), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("listeners: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Mihomo: config.MihomoConfig{Binary: binary, Config: path}}
	s := NewService(cfg)
	if err := s.updater.prepareDirectory(); err != nil {
		t.Fatal(err)
	}
	return s, cfg
}
func fakeCore(version, body string, invalid bool) string {
	validation := "exit 0"
	if invalid {
		validation = "echo invalid-config >&2; exit 1"
	}
	return fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n-v) echo 'Mihomo %s'; exit 0;;\n-t) %s;;\nesac\n%s\n", version, validation, body)
}
func publishFixture(t *testing.T, s *Service, version, body string, invalid bool) *coreArtifact {
	t.Helper()
	a, err := s.updater.publishBinary(strings.NewReader(fakeCore(version, body, invalid)), version)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func pendingRollback(s *Service, a *coreArtifact) {
	s.updater.selection.Previous = a
	s.updater.job = &CoreUpdateJob{Status: "running", Stage: "preparing"}
}
func TestCoreReleaseFilter(t *testing.T) {
	base := githubRelease{Tag: "v1.19.30"}
	if err := json.Unmarshal([]byte(`{"assets":[{"name":"mihomo-linux-arm64-v1.19.30.gz","digest":"sha256:`+strings.Repeat("a", 64)+`","size":100}]}`), &base); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*githubRelease)
		ok     bool
	}{
		{"stable", func(*githubRelease) {}, true},
		{"prerelease", func(r *githubRelease) { r.Prerelease = true }, false},
		{"draft", func(r *githubRelease) { r.Draft = true }, false},
		{"path", func(r *githubRelease) { r.Tag = "../../etc" }, false},
		{"missing digest", func(r *githubRelease) { r.Assets[0].Digest = "" }, false},
		{"wrong architecture", func(r *githubRelease) { r.Assets[0].Name = "mihomo-linux-amd64-v1-v1.19.30.gz" }, false},
		{"oversized", func(r *githubRelease) { r.Assets[0].Size = maxArchiveBytes + 1 }, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var r githubRelease
			_ = json.Unmarshal(raw, &r)
			tt.change(&r)
			_, ok := officialRelease(r, "linux-arm64")
			if ok != tt.ok {
				t.Fatalf("got %v", ok)
			}
		})
	}
}
func TestCoreSelectionIntegrity(t *testing.T) {
	s, cfg := fixtureCore(t, "exec sleep 60")
	active := publishFixture(t, s, "v2.0.0", "exec sleep 60", false)
	if err := s.updater.saveSelection(coreSelection{Schema: 1, Active: active}); err != nil {
		t.Fatal(err)
	}
	reloaded := NewService(cfg)
	if reloaded.BinaryPath() != s.updater.artifactPath(active) {
		t.Fatal("selected binary lost on restart")
	}
	// Updating the image's bundled core must not reset a persisted selection.
	if err := os.WriteFile(cfg.Mihomo.Binary, []byte(fakeCore("v3.0.0", "exec sleep 60", false)), 0700); err != nil {
		t.Fatal(err)
	}
	if got := NewService(cfg).BinaryPath(); got != reloaded.BinaryPath() {
		t.Fatal("image reset selected core")
	}
	if err := os.WriteFile(reloaded.BinaryPath(), []byte("corrupted"), 0700); err != nil {
		t.Fatal(err)
	}
	broken := NewService(cfg)
	if broken.updater.initErr == nil || broken.StartMihomo() == nil {
		t.Fatal("corrupt selected core must fail closed")
	}
}
func TestCoreSelectionRejectsSymlinkAndTraversal(t *testing.T) {
	s, _ := fixtureCore(t, "exit 0")
	if err := s.updater.verifyArtifact(&coreArtifact{Version: "v1.0.0", SHA256: "../../outside"}); err == nil {
		t.Fatal("accepted traversal")
	}
	a := publishFixture(t, s, "v2.0.0", "exit 0", false)
	path := s.updater.artifactPath(a)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(s.pm.BinaryPath(), path); err != nil {
		t.Fatal(err)
	}
	if err := s.updater.verifyArtifact(a); err == nil {
		t.Fatal("accepted symlink")
	}
}
func TestStoppedCoreRollbackAndRestartPersistence(t *testing.T) {
	s, cfg := fixtureCore(t, "exec sleep 60")
	candidate := publishFixture(t, s, "v2.0.0", "exec sleep 60", false)
	pendingRollback(s, candidate)
	if restored, err := s.performCoreUpdate("v2.0.0", true); err != nil || restored {
		t.Fatalf("%v %v", restored, err)
	}
	if s.pm.IsRunning() {
		t.Fatal("stopped core was started")
	}
	if s.updater.selection.Previous.Version != "v1.0.0" {
		t.Fatal("old core not backed up")
	}
	if NewService(cfg).BinaryPath() != s.pm.BinaryPath() {
		t.Fatal("successful selection not persisted")
	}
	pendingRollback(s, s.updater.selection.Previous)
	if _, err := s.performCoreUpdate("v1.0.0", true); err != nil {
		t.Fatal(err)
	}
	info, err := s.pm.GetVersion()
	if err != nil || info.Version != "v1.0.0" {
		t.Fatalf("%v %v", info, err)
	}
}
func TestInvalidCandidateLeavesCurrentSelectionUntouched(t *testing.T) {
	for _, kind := range []string{"invalid config", "wrong version"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := fixtureCore(t, "exec sleep 60")
			candidate := publishFixture(t, s, "v2.0.0", "exit 1", kind == "invalid config")
			pendingRollback(s, candidate)
			before := s.pm.BinaryPath()
			version := "v2.0.0"
			if kind == "wrong version" {
				version = "v9.0.0"
			}
			if _, err := s.performCoreUpdate(version, true); err == nil {
				t.Fatal("invalid candidate accepted")
			}
			if s.pm.BinaryPath() != before {
				t.Fatal("changed old core before validation")
			}
			if _, err := os.Stat(filepath.Join(s.updater.dir, "selection.json")); !os.IsNotExist(err) {
				t.Fatal("persisted invalid candidate")
			}
		})
	}
}
func TestCoreUpdateRejectsConcurrentMutations(t *testing.T) {
	s, _ := fixtureCore(t, "exit 0")
	s.updater.job = &CoreUpdateJob{Status: "running"}
	for name, fn := range map[string]func() error{"start": s.StartMihomo, "stop": s.StopMihomo, "restart": s.RestartMihomo, "save": func() error { return s.SaveConfig("listeners: []") }, "apply": func() error { return s.ApplyConfig("listeners: []") }} {
		if err := fn(); err != ErrCoreUpdateBusy {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if runtime.GOOS == "linux" {
		if _, err := s.BeginCoreUpdate("v2.0.0", false); err != ErrCoreUpdateBusy {
			t.Fatalf("duplicate: %v", err)
		}
	}
}
func TestCoreSwitchRecoversStartupFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("socket ownership inspection needs Linux")
	}
	for _, body := range []string{"exit 1", "sleep 0.7; exit 1"} {
		t.Run(body, func(t *testing.T) {
			s, cfg := fixtureCore(t, "exec sleep 60")
			if err := s.pm.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.pm.quiesce() })
			candidate := publishFixture(t, s, "v2.0.0", body, false)
			pendingRollback(s, candidate)
			restored, err := s.performCoreUpdate("v2.0.0", true)
			if err == nil || !restored || !s.pm.IsRunning() {
				t.Fatalf("restored=%v running=%v err=%v", restored, s.pm.IsRunning(), err)
			}
			selected := NewService(cfg)
			info, err := selected.pm.GetVersion()
			if err != nil || info.Version != "v1.0.0" {
				t.Fatalf("candidate persisted after failure: %v %v", info, err)
			}
			_ = s.pm.quiesce()
			time.Sleep(2200 * time.Millisecond)
			if s.pm.IsRunning() {
				t.Fatal("crashed candidate's delayed restart revived the core")
			}
		})
	}
}
func TestCoreDownloadChecksumAndLimits(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("official release downloads are Linux only")
	}
	for _, kind := range []string{"valid", "checksum", "size", "metadata status", "gzip", "version tag"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := fixtureCore(t, "exec sleep 60")
			s.updater.job = &CoreUpdateJob{Status: "running"}
			var compressed bytes.Buffer
			gz := gzip.NewWriter(&compressed)
			_, _ = gz.Write([]byte(fakeCore("v2.0.0", "exec sleep 60", false)))
			_ = gz.Close()
			archive := compressed.Bytes()
			if kind == "gzip" {
				archive = []byte("not gzip")
			}
			sum := sha256.Sum256(archive)
			digest := hex.EncodeToString(sum[:])
			if kind == "checksum" {
				digest = strings.Repeat("0", 64)
			}
			size := len(archive)
			if kind == "size" {
				size++
			}
			platform, _ := corePlatform()
			tag := "v2.0.0"
			if kind == "version tag" {
				tag = "v9.0.0"
			}
			metadata := fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":%q,"digest":%q,"size":%d}]}`, tag, "mihomo-"+platform+"-"+tag+".gz", "sha256:"+digest, size)
			s.updater.client = &http.Client{Transport: coreRoundTrip(func(r *http.Request) (*http.Response, error) {
				body := archive
				code := 200
				if r.URL.Host == "api.github.com" {
					body = []byte(metadata)
					if kind == "metadata status" {
						code = 403
					}
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}
			a, err := s.downloadCore("v2.0.0")
			if (err == nil) != (kind == "valid") {
				t.Fatalf("artifact=%v error=%v", a, err)
			}
			if err == nil {
				if err := s.updater.verifyArtifact(a); err != nil {
					t.Fatal(err)
				}
			}
			files, _ := filepath.Glob(filepath.Join(s.updater.dir, ".download-*"))
			if len(files) != 0 {
				t.Fatal("download not cleaned up")
			}
		})
	}
}
func TestCoreRedirectAllowlist(t *testing.T) {
	client := coreHTTPClient()
	for _, url := range []string{"http://github.com/file", "https://evil.example/file", "https://github.com@evil.example/file", "https://127.0.0.1/file"} {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
		if err := client.CheckRedirect(req, nil); err == nil {
			t.Fatalf("accepted %s", url)
		}
	}
	req, _ := http.NewRequest("GET", "https://release-assets.githubusercontent.com/file", nil)
	if err := client.CheckRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCoreUpdateHonorsDeploymentLock(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux updater")
	}
	s, _ := fixtureCore(t, "exit 0")
	f, err := os.OpenFile(filepath.Join(s.updater.dir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginCoreUpdate("v2.0.0", false); err != ErrCoreUpdateBusy {
		t.Fatalf("deployment lock ignored: %v", err)
	}
	if s.CoreUpdateStatus().Busy {
		t.Fatal("rejected job remained active")
	}
}
func TestInterruptedCoreJobSurvivesRestart(t *testing.T) {
	s, cfg := fixtureCore(t, "exit 0")
	s.updater.job = &CoreUpdateJob{ID: "interrupted-job", Version: "v2.0.0", Status: "running", Stage: "switching"}
	if err := s.updater.saveJob(); err != nil {
		t.Fatal(err)
	}
	reloaded := NewService(cfg)
	status := reloaded.CoreUpdateStatus()
	if status.Job == nil || status.Job.Status != "failed" || status.Job.Stage != "interrupted" || status.Busy {
		t.Fatalf("interrupted job lost: %+v", status)
	}
	if reloaded.pm.BinaryPath() != cfg.Mihomo.Binary {
		t.Fatal("unfinished candidate selected")
	}
}
func TestCoreReadinessRejectsMissingSocket(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux socket ownership")
	}
	s, _ := fixtureCore(t, "exec sleep 60")
	if err := s.pm.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.pm.quiesce()
	content := "listeners:\n- name: absent\n  type: vless\n  listen: 127.0.0.1\n  port: '65123'\n"
	if err := s.checkUpdateReadiness(content); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("missing listener accepted: %v", err)
	}
}

func TestCoreMetadataAllowsRealisticReleaseListsAndBoundsInput(t *testing.T) {
	for _, n := range []int{5 << 20, maxReleaseMetadataBytes + 1} {
		u := &coreUpdater{client: &http.Client{Transport: coreRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat(" ", n) + "[]"))}, nil
		})}}
		var out []githubRelease
		err := u.readReleaseJSON(context.Background(), releaseAPI, &out)
		if (err != nil) != (n > maxReleaseMetadataBytes) {
			t.Fatalf("size %d: %v", n, err)
		}
	}
}

func TestCoreUpdateReconcilesCommittedCredentials(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed=%v", failed), func(t *testing.T) {
			s, _ := fixtureCore(t, "exec sleep 60")
			db, err := database.InitDB(filepath.Join(t.TempDir(), "credentials.db"))
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			security.InitCredentialKey("core-update-test")
			users := user.NewService(db)
			listener := models.Listener{Name: "fixture", Protocol: "shadowsocks", Port: "18388", Enabled: true}
			if err := db.Create(&listener).Error; err != nil {
				t.Fatal(err)
			}
			var ids []uint
			for _, name := range []string{"deleted", "rotated", "unbound"} {
				u, err := users.Create(user.CreateInput{Username: name, Password: "old-password"})
				if err != nil {
					t.Fatal(err)
				}
				if err := users.BindListeners(u.ID, []uint{listener.ID}); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, u.ID)
			}
			regenerations := 0
			regenerate := func() error {
				regenerations++
				credentials, err := users.ActiveCredentialsByListener()
				if err != nil {
					return err
				}
				content, err := yaml.Marshal(map[string]any{"credentials": credentials})
				if err != nil {
					return err
				}
				return s.SaveConfig(string(content))
			}
			users.SetCredentialsChangedHandler(func() error { return s.SyncCredentials(regenerate) })
			if err := regenerate(); err != nil {
				t.Fatal(err)
			}
			s.updater.job = &CoreUpdateJob{Status: "running"}
			if err := users.Delete(ids[0]); !errors.Is(err, ErrCoreUpdateBusy) {
				t.Fatalf("delete: %v", err)
			}
			if _, err := users.Update(ids[1], user.UpdateInput{Password: "new-password"}); !errors.Is(err, ErrCoreUpdateBusy) {
				t.Fatalf("rotate: %v", err)
			}
			if err := users.BindListeners(ids[2], nil); !errors.Is(err, ErrCoreUpdateBusy) {
				t.Fatalf("unbind: %v", err)
			}
			var updateErr error
			if failed {
				updateErr = fmt.Errorf("download failed")
			}
			s.finishCoreUpdate(false, updateErr)
			got, err := s.cm.ReadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "new-password") || strings.Contains(got, "old-password") || strings.Contains(got, "deleted") || strings.Contains(got, "unbound") {
				t.Fatalf("committed credential changes not reconciled: %s", got)
			}
			if regenerations != 2 {
				t.Fatalf("expected one coalesced regeneration after the update, got %d", regenerations)
			}
		})
	}
}

func TestCredentialSyncHandlesUpdateFinishingDuringRegeneration(t *testing.T) {
	s, _ := fixtureCore(t, "exit 0")
	calls := 0
	err := s.SyncCredentials(func() error {
		calls++
		if calls == 1 {
			s.updater.job = &CoreUpdateJob{Status: "running"}
			s.finishCoreUpdate(false, nil)
			return ErrCoreUpdateBusy
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("lost retry: calls=%d err=%v", calls, err)
	}
}

func TestCredentialSyncRequeuesForNextUpdate(t *testing.T) {
	s, _ := fixtureCore(t, "exit 0")
	s.updater.job = &CoreUpdateJob{Status: "running"}
	calls := 0
	regenerate := func() error {
		calls++
		if calls == 1 {
			s.updater.job = &CoreUpdateJob{Status: "running"}
			return ErrCoreUpdateBusy
		}
		return nil
	}
	if err := s.SyncCredentials(regenerate); !errors.Is(err, ErrCoreUpdateBusy) {
		t.Fatal(err)
	}
	s.finishCoreUpdate(false, nil)
	if calls != 1 || s.updater.pendingCredentials == nil {
		t.Fatal("lost deferred synchronization for next update")
	}
	s.finishCoreUpdate(false, fmt.Errorf("second update failed"))
	if calls != 2 || s.updater.pendingCredentials != nil {
		t.Fatal("did not drain deferred synchronization")
	}
}

func TestCoreUpdateListenerHelper(t *testing.T) {
	port := os.Getenv("THREE_M_UI_TEST_LISTENER")
	if port == "" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}
}

func TestManualCoreRollbackRecoversMissingCurrentListener(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux socket ownership inspection")
	}
	reserve, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	s, cfg := fixtureCore(t, "exec sleep 60")
	content := fmt.Sprintf("listeners:\n- name: recovery\n  type: vless\n  listen: 127.0.0.1\n  port: '%d'\n", port)
	if err := os.WriteFile(cfg.Mihomo.Config, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.pm.quiesce() })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("exec env THREE_M_UI_TEST_LISTENER=%d %s -test.run=^TestCoreUpdateListenerHelper$", port, "'"+strings.ReplaceAll(executable, "'", "'\\''")+"'")
	candidate := publishFixture(t, s, "v2.0.0", body, false)
	pendingRollback(s, candidate)
	restored, err := s.performCoreUpdate("v2.0.0", true)
	if err != nil || restored {
		t.Fatalf("rollback from unhealthy core: restored=%v err=%v", restored, err)
	}
	if s.pm.BinaryPath() != s.updater.artifactPath(candidate) || !s.pm.IsRunning() {
		t.Fatal("previous version not activated")
	}
	if err := s.checkUpdateReadiness(content); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialAlphaRelease(t *testing.T) {
	for _, platform := range []string{"linux-arm64", "linux-amd64-v1"} {
		for _, kind := range []string{"valid", "draft", "other tag", "not pre", "missing digest", "alternate Go", "wrong platform", "ambiguous"} {
			t.Run(platform+"/"+kind, func(t *testing.T) {
				var r githubRelease
				raw := fmt.Sprintf(`{"tag_name":"Prerelease-Alpha","prerelease":true,"assets":[{"name":"mihomo-%s-alpha-dca26db.gz","digest":"sha256:%s","size":100}]}`, platform, strings.Repeat("a", 64))
				if err := json.Unmarshal([]byte(raw), &r); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "draft":
					r.Draft = true
				case "other tag":
					r.Tag = "Prerelease-Beta"
				case "not pre":
					r.Prerelease = false
				case "missing digest":
					r.Assets[0].Digest = ""
				case "alternate Go":
					r.Assets[0].Name = "mihomo-" + platform + "-go123-alpha-dca26db.gz"
				case "wrong platform":
					r.Assets[0].Name = "mihomo-windows-arm64-alpha-dca26db.gz"
				case "ambiguous":
					r.Assets = append(r.Assets, r.Assets[0])
					r.Assets[1].Name = "mihomo-" + platform + "-alpha-123abcd.gz"
				}
				got, ok := officialRelease(r, platform)
				if ok != (kind == "valid") {
					t.Fatalf("release=%+v ok=%v", got, ok)
				}
				if ok && (got.Version != "alpha-dca26db" || !got.Prerelease || !strings.HasSuffix(got.URL, "/Prerelease-Alpha")) {
					t.Fatalf("%+v", got)
				}
			})
		}
	}
}

func TestAlphaInstallPersistenceAndRollingRelease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux updater")
	}
	for _, kind := range []string{"valid", "replaced build", "wrong executable", "checksum"} {
		t.Run(kind, func(t *testing.T) {
			s, cfg := fixtureCore(t, "exec sleep 60")
			s.updater.job = &CoreUpdateJob{Status: "running"}
			version := "alpha-dca26db"
			executableVersion := version
			if kind == "wrong executable" {
				executableVersion = "alpha-123abcd"
			}
			var archive bytes.Buffer
			gz := gzip.NewWriter(&archive)
			_, _ = gz.Write([]byte(fakeCore(executableVersion, "exec sleep 60", false)))
			_ = gz.Close()
			sum := sha256.Sum256(archive.Bytes())
			digest := hex.EncodeToString(sum[:])
			if kind == "checksum" {
				digest = strings.Repeat("0", 64)
			}
			assetVersion := version
			if kind == "replaced build" {
				assetVersion = "alpha-123abcd"
			}
			platform, _ := corePlatform()
			asset := "mihomo-" + platform + "-" + assetVersion + ".gz"
			metadata := fmt.Sprintf(`{"tag_name":"Prerelease-Alpha","prerelease":true,"assets":[{"name":%q,"digest":%q,"size":%d}]}`, asset, "sha256:"+digest, archive.Len())
			downloads := 0
			s.updater.client = &http.Client{Transport: coreRoundTrip(func(r *http.Request) (*http.Response, error) {
				body := archive.Bytes()
				switch r.URL.String() {
				case releaseAPI + "/tags/Prerelease-Alpha":
					body = []byte(metadata)
				case releaseDownload + "Prerelease-Alpha/" + asset:
					downloads++
				default:
					t.Fatalf("unexpected URL %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}
			before := s.pm.BinaryPath()
			_, err := s.performCoreUpdate(version, false)
			if kind != "valid" {
				if err == nil || s.pm.BinaryPath() != before {
					t.Fatalf("unsafe switch: %v", err)
				}
				if kind == "replaced build" && downloads != 0 {
					t.Fatal("downloaded a different build")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s.pm.IsRunning() {
				t.Fatal("started a stopped core")
			}
			reloaded := NewService(cfg)
			info, err := reloaded.pm.GetVersion()
			if err != nil || info.Version != version || reloaded.updater.selection.Active.Version != version {
				t.Fatalf("lost concrete pre build: %+v %v", info, err)
			}
			for _, target := range []string{"v1.0.0", version} {
				pendingRollback(s, s.updater.selection.Previous)
				if _, err := s.performCoreUpdate(target, true); err != nil {
					t.Fatal(err)
				}
				info, err := s.pm.GetVersion()
				if err != nil || info.Version != target {
					t.Fatalf("rollback: %+v %v", info, err)
				}
			}
			if downloads != 1 {
				t.Fatal("rollback used the network")
			}
		})
	}
}

func TestAlphaReleaseListRefresh(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux updater")
	}
	s, _ := fixtureCore(t, "exit 0")
	platform, _ := corePlatform()
	version := "alpha-dca26db"
	requests := 0
	s.updater.client = &http.Client{Transport: coreRoundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != releaseAPI+"?per_page=30" {
			t.Fatalf("unexpected URL: %s", r.URL)
		}
		body := fmt.Sprintf(`[{"tag_name":"Prerelease-Alpha","prerelease":true,"assets":[{"name":"mihomo-%s-%s.gz","digest":"sha256:%s","size":100}]},{"tag_name":"v1.19.30","assets":[{"name":"mihomo-%s-v1.19.30.gz","digest":"sha256:%s","size":100}]}]`, platform, version, strings.Repeat("a", 64), platform, strings.Repeat("b", 64))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	for i := 0; i < 3; i++ {
		if i == 2 {
			version = "alpha-123abcd"
			s.updater.releasesAt = time.Now().Add(-2 * time.Minute)
		}
		releases, err := s.CoreReleases(context.Background())
		if err != nil || len(releases) != 2 || releases[0].Prerelease || releases[1].Version != version {
			t.Fatalf("list=%+v err=%v", releases, err)
		}
	}
	if requests != 2 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestRealAlphaCoreSwitchAndRollback(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_PRE_BINARY")
	if runtime.GOOS != "linux" || binary == "" {
		t.Skip("set MIHOMO_TEST_PRE_BINARY for isolated real pre tests")
	}
	s, cfg := fixtureCore(t, "exec sleep 60")
	stable := os.Getenv("MIHOMO_TEST_BINARY")
	if stable == "" {
		t.Fatal("MIHOMO_TEST_BINARY is required")
	}
	data, err := os.ReadFile(stable)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(cfg.Mihomo.Binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	stableInfo, err := s.pm.GetVersion()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.pm.quiesce() })
	data, err = os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := s.updater.publishBinary(bytes.NewReader(data), "pre-probe")
	if err != nil {
		t.Fatal(err)
	}
	probe := NewProcessManager(s.updater.artifactPath(candidate), cfg.Mihomo.Config)
	probe.managedDir = s.updater.dir
	info, err := probe.GetVersion()
	if err != nil || !alphaVersion.MatchString(info.Version) {
		t.Fatalf("pre version=%+v err=%v", info, err)
	}
	candidate.Version = info.Version
	for _, target := range []string{info.Version, stableInfo.Version, info.Version} {
		pendingRollback(s, candidate)
		if _, err = s.performCoreUpdate(target, true); err != nil {
			t.Fatal(err)
		}
		if !s.pm.IsRunning() {
			t.Fatal("running state lost")
		}
		reloaded := NewService(cfg)
		got, err := reloaded.pm.GetVersion()
		if err != nil || got.Version != target {
			t.Fatalf("persisted version=%+v err=%v", got, err)
		}
		candidate = s.updater.selection.Previous
	}
	t.Logf("real stable %s ↔ pre %s switch, persistence and rollback passed", stableInfo.Version, info.Version)
}
