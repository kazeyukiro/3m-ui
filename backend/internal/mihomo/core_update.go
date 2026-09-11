package mihomo

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var ErrCoreUpdateBusy = fmt.Errorf("a core update is in progress; wait for it to finish")

type CoreUpdateJob struct {
	ID         string     `json:"id"`
	Operation  string     `json:"operation"`
	Version    string     `json:"version"`
	Status     string     `json:"status"`
	Stage      string     `json:"stage"`
	Error      string     `json:"error,omitempty"`
	RolledBack bool       `json:"rolled_back"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
type CoreUpdateStatus struct {
	Supported       bool           `json:"supported"`
	DisabledReason  string         `json:"disabled_reason,omitempty"`
	Busy            bool           `json:"busy"`
	Managed         bool           `json:"managed"`
	PreviousVersion string         `json:"previous_version,omitempty"`
	Job             *CoreUpdateJob `json:"job,omitempty"`
}
type coreUpdater struct {
	lock       *os.File
	mu         sync.Mutex
	dir        string
	selection  coreSelection
	job        *CoreUpdateJob
	initErr    error
	client     *http.Client
	releaseMu  sync.Mutex
	releases   []CoreRelease
	releasesAt time.Time
	// Regenerate from the latest database after the job, never replay old YAML.
	pendingCredentials func() error
}

// SyncCredentials preserves a committed user/binding change when a core update
// temporarily rejects ApplyConfig. Other configuration edits still fail fast.
func (s *Service) SyncCredentials(regenerate func() error) error {
	if s.updater == nil {
		return regenerate()
	}
	u := s.updater
	for {
		u.mu.Lock()
		if u.job != nil && u.job.Status == "running" {
			u.pendingCredentials = regenerate
			u.mu.Unlock()
			return ErrCoreUpdateBusy
		}
		u.mu.Unlock()
		if err := regenerate(); !errors.Is(err, ErrCoreUpdateBusy) {
			return err
		}
		// An update began during regeneration. Queue under the same mutex used
		// to finish it, or retry if it has already finished, so no wakeup is lost.
	}
}

func (s *Service) initCoreUpdater(configPath string) {
	if configPath == "" {
		return
	}
	dir, err := filepath.Abs(filepath.Join(filepath.Dir(configPath), "cores"))
	u := &coreUpdater{dir: dir, client: coreHTTPClient(), initErr: err}
	if err == nil {
		u.initErr = u.loadSelection()
	}
	if u.initErr == nil {
		if raw, err := os.ReadFile(filepath.Join(dir, "job.json")); err == nil && len(raw) < 65536 {
			var job CoreUpdateJob
			if json.Unmarshal(raw, &job) == nil && job.ID != "" {
				if job.Status == "running" {
					now := time.Now().UTC()
					job.Status = "failed"
					job.Stage = "interrupted"
					job.Error = "Panel restarted during an update; check the selected core version before retrying"
					job.FinishedAt = &now
				}
				u.job = &job
			}
		}
	}
	s.updater = u
	s.pm.managedDir = dir
	if u.initErr == nil && u.selection.Active != nil {
		s.pm.binaryPath = u.artifactPath(u.selection.Active)
	}
}
func (s *Service) CoreUpdateStatus() CoreUpdateStatus {
	result := CoreUpdateStatus{}
	if s == nil || s.updater == nil {
		result.DisabledReason = "core updates are unavailable"
		return result
	}
	u := s.updater
	u.mu.Lock()
	defer u.mu.Unlock()
	_, err := corePlatform()
	if u.initErr != nil {
		err = fmt.Errorf("saved core selection is unavailable: %w", u.initErr)
	}
	if err != nil {
		result.DisabledReason = err.Error()
	} else {
		result.Supported = true
	}
	result.Managed = u.selection.Active != nil
	if u.selection.Previous != nil {
		result.PreviousVersion = u.selection.Previous.Version
	}
	if u.job != nil {
		copy := *u.job
		result.Job = &copy
		result.Busy = copy.Status == "running"
	}
	return result
}
func (s *Service) coreMutationAllowed() error {
	if s.updater == nil {
		return nil
	}
	u := s.updater
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.initErr != nil {
		return fmt.Errorf("saved core selection is unavailable: %w", u.initErr)
	}
	if u.job != nil && u.job.Status == "running" {
		return ErrCoreUpdateBusy
	}
	return nil
}

// BeginCoreUpdate returns immediately; the job lives independently of the HTTP
// request, so navigating away cannot interrupt the switch or its recovery.
func (s *Service) BeginCoreUpdate(version string, rollback bool) (*CoreUpdateJob, error) {
	if s == nil || s.updater == nil {
		return nil, fmt.Errorf("core updates are unavailable")
	}
	if _, err := corePlatform(); err != nil {
		return nil, err
	}
	if !rollback && !stableVersion.MatchString(version) && !alphaVersion.MatchString(version) {
		return nil, fmt.Errorf("select an official stable or pre version")
	}
	u := s.updater
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.initErr != nil {
		return nil, fmt.Errorf("saved core selection is unavailable: %w", u.initErr)
	}
	if u.job != nil && u.job.Status == "running" {
		return nil, ErrCoreUpdateBusy
	}
	operation := "install"
	if rollback {
		if u.selection.Previous == nil {
			return nil, fmt.Errorf("no previous core version is available")
		}
		operation = "rollback"
		version = u.selection.Previous.Version
	}
	if err := u.prepareDirectory(); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(u.dir, "update.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("open core update lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, ErrCoreUpdateBusy
	}
	accepted := false
	defer func() {
		if !accepted {
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
			lock.Close()
		}
	}()
	// Check write permissions before returning an accepted job.
	f, err := os.CreateTemp(u.dir, ".writable-*")
	if err != nil {
		return nil, fmt.Errorf("persistent core directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(f.Name())
	now := time.Now().UTC()
	u.job = &CoreUpdateJob{ID: fmt.Sprintf("%d", now.UnixNano()), Operation: operation, Version: version, Status: "running", Stage: "preparing", StartedAt: now}
	if err := u.saveJob(); err != nil {
		u.job = nil
		return nil, err
	}
	u.lock = lock
	accepted = true
	job := *u.job
	go s.runCoreUpdate(version, rollback)
	return &job, nil
}
func (u *coreUpdater) stage(stage string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.job.Stage = stage
	_ = u.saveJob()
}
func (s *Service) runCoreUpdate(version string, rollback bool) {
	rolledBack, err := s.performCoreUpdate(version, rollback)
	s.finishCoreUpdate(rolledBack, err)
}
func (s *Service) finishCoreUpdate(rolledBack bool, err error) {
	u := s.updater
	u.mu.Lock()
	now := time.Now().UTC()
	u.job.FinishedAt = &now
	u.job.RolledBack = rolledBack
	if err != nil {
		u.job.Status = "failed"
		u.job.Error = err.Error()
	} else {
		u.job.Status = "succeeded"
		u.job.Stage = "complete"
	}
	_ = u.saveJob()
	// Retain selected/previous binaries and the current process path even when
	// recovery failed. Invalid candidates must not accumulate across retries.
	u.pruneBinaries(s.pm.BinaryPath())
	if u.lock != nil {
		_ = syscall.Flock(int(u.lock.Fd()), syscall.LOCK_UN)
		_ = u.lock.Close()
		u.lock = nil
	}
	pending := u.pendingCredentials
	u.pendingCredentials = nil
	u.mu.Unlock()
	// The callback takes the listener/config locks and may queue itself for a
	// newer update. Never invoke it while holding the updater mutex.
	if pending != nil {
		if err := s.SyncCredentials(pending); err != nil && !errors.Is(err, ErrCoreUpdateBusy) {
			log.Printf("Mihomo credential synchronization after core update failed: %v", err)
		}
	}
}
func (s *Service) downloadCore(version string) (*coreArtifact, error) {
	u := s.updater
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	platform, err := corePlatform()
	if err != nil {
		return nil, err
	}
	u.stage("resolving")
	tag := version
	if alphaVersion.MatchString(version) {
		tag = alphaReleaseTag
	} else if !stableVersion.MatchString(version) {
		return nil, fmt.Errorf("select an official stable or pre version")
	}
	var metadata githubRelease
	if err := u.readReleaseJSON(ctx, releaseAPI+"/tags/"+tag, &metadata); err != nil {
		return nil, err
	}
	release, ok := officialRelease(metadata, platform)
	if !ok || release.Version != version {
		return nil, fmt.Errorf("selected release is no longer available or has no verifiable artifact for this platform; check for updates again")
	}
	u.stage("downloading")
	resp, err := u.get(ctx, releaseDownload+tag+"/"+release.Asset)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	f, err := os.CreateTemp(u.dir, ".download-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if n != release.Size || n > maxArchiveBytes {
		return nil, fmt.Errorf("release archive size mismatch")
	}
	u.stage("verifying")
	if hex.EncodeToString(h.Sum(nil)) != release.SHA256 {
		return nil, fmt.Errorf("release archive SHA-256 mismatch")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return u.publishBinary(gz, version)
}
func (s *Service) performCoreUpdate(version string, rollback bool) (bool, error) {
	u := s.updater
	var candidate *coreArtifact
	var err error
	if rollback {
		u.mu.Lock()
		candidate = u.selection.Previous
		u.mu.Unlock()
		if err = u.verifyArtifact(candidate); err != nil {
			return false, fmt.Errorf("previous core unavailable: %w", err)
		}
	} else {
		candidate, err = s.downloadCore(version)
		if err != nil {
			return false, err
		}
	}
	// Serialize config writes and process lifecycle only after download. The
	// public mutation methods reject changes during the job instead of queuing
	// a stale operation that unexpectedly runs once an update finishes.
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	u.stage("validating")
	check := NewProcessManager(u.artifactPath(candidate), s.pm.configPath)
	check.managedDir = u.dir
	info, err := check.GetVersion()
	if err != nil {
		return false, fmt.Errorf("candidate is not a runnable Mihomo core: %w", err)
	}
	if info.Version != version {
		return false, fmt.Errorf("candidate version does not match selected release")
	}
	if err := check.ValidateConfig(); err != nil {
		return false, err
	}
	content, err := s.cm.ReadConfig()
	if err != nil {
		return false, err
	}
	wasRunning := s.pm.IsRunning()
	if wasRunning && !rollback {
		if err := s.checkUpdateReadiness(content); err != nil {
			return false, fmt.Errorf("current core is not healthy; resolve its listener errors before updating: %w", err)
		}
	}
	oldPath := s.pm.BinaryPath()
	oldVersion, err := s.pm.GetVersion()
	if err != nil {
		return false, fmt.Errorf("read current core version: %w", err)
	}
	oldFile, err := os.Open(oldPath)
	if err != nil {
		return false, err
	}
	previous, err := u.publishBinary(oldFile, oldVersion.Version)
	oldFile.Close()
	if err != nil {
		return false, fmt.Errorf("back up current core: %w", err)
	}
	u.mu.Lock()
	oldState := u.selection
	u.mu.Unlock()
	// Snapshot the initial bundled core as well, so rollback still works after a
	// future panel image changes its bundled version.
	if oldState.Active == nil {
		oldState = coreSelection{Schema: 1, Active: previous}
		if err := u.saveSelection(oldState); err != nil {
			return false, err
		}
		u.mu.Lock()
		u.selection = oldState
		u.mu.Unlock()
	}
	if candidate.SHA256 == previous.SHA256 {
		return false, nil
	}
	u.stage("switching")
	if err := s.pm.quiesce(); err != nil {
		return false, err
	}
	s.pm.setBinaryPath(u.artifactPath(candidate))
	recoverOld := func(cause error) (bool, error) {
		u.stage("recovering")
		if err := s.pm.quiesce(); err != nil {
			return false, fmt.Errorf("%v; could not stop candidate: %w", cause, err)
		}
		s.pm.setBinaryPath(u.artifactPath(previous))
		if wasRunning {
			if err := s.pm.Start(); err != nil {
				return false, fmt.Errorf("%v; previous core restart failed: %w", cause, err)
			}
			if err := s.checkUpdateReadiness(content); err != nil {
				return false, fmt.Errorf("%v; previous core recovery check failed: %w", cause, err)
			}
		}
		return true, fmt.Errorf("%v; restored previous core", cause)
	}
	if wasRunning {
		u.stage("starting")
		if err := s.pm.Start(); err != nil {
			return recoverOld(err)
		}
		u.stage("checking")
		if err := s.checkUpdateReadiness(content); err != nil {
			return recoverOld(err)
		}
	}
	next := coreSelection{Schema: 1, Active: candidate, Previous: previous}
	if err := u.saveSelection(next); err != nil {
		return recoverOld(fmt.Errorf("save core selection: %w", err))
	}
	u.mu.Lock()
	u.selection = next
	u.mu.Unlock()
	return false, nil
}

// Require actual inspection for updates. Ordinary ApplyConfig intentionally
// treats unavailable socket inspection as unknown; an updater must not commit
// a new binary based on that absence of evidence.
func (s *Service) checkUpdateReadiness(content string) error {
	listeners, err := runtimeListeners(content)
	if err != nil {
		return err
	}
	expected := make([][]RuntimeEndpoint, len(listeners))
	for i, l := range listeners {
		expected[i], _ = l.endpoints()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var inspectErr error
	inspect := func(ctx context.Context, pid int) ([]runtimeConnection, error) {
		rows, err := ownedConnections(ctx, pid)
		inspectErr = err
		return rows, err
	}
	if err := waitForListenerSockets(ctx, listeners, expected, s.runtimePID, inspect); err != nil {
		return err
	}
	if inspectErr != nil {
		return fmt.Errorf("could not verify core listening sockets: %w", inspectErr)
	}
	// Catch immediate exits even when there are no configured listeners.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	<-timer.C
	if !s.pm.IsRunning() {
		return fmt.Errorf("Mihomo stopped during readiness verification")
	}
	if err := waitForListenerSockets(ctx, listeners, expected, s.runtimePID, inspect); err != nil {
		return err
	}
	if inspectErr != nil {
		return fmt.Errorf("could not verify core listening sockets: %w", inspectErr)
	}
	return nil
}
