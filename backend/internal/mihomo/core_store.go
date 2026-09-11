package mihomo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type coreArtifact struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type coreSelection struct {
	Schema   int           `json:"schema"`
	Active   *coreArtifact `json:"active"`
	Previous *coreArtifact `json:"previous,omitempty"`
}

func (u *coreUpdater) artifactPath(a *coreArtifact) string {
	return filepath.Join(u.dir, "mihomo-"+a.SHA256)
}
func (u *coreUpdater) verifyArtifact(a *coreArtifact) error {
	if a == nil || !sha256Pattern.MatchString(a.SHA256) || a.Version == "" {
		return fmt.Errorf("invalid saved core version")
	}
	path := u.artifactPath(a)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Size() > maxBinaryBytes {
		return fmt.Errorf("saved core is not a regular executable")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return fmt.Errorf("saved core checksum mismatch")
	}
	return nil
}
func (u *coreUpdater) prepareDirectory() error {
	if err := os.MkdirAll(u.dir, 0700); err != nil {
		return fmt.Errorf("create persistent core directory: %w", err)
	}
	real, err := filepath.EvalSymlinks(u.dir)
	if err != nil || real != u.dir {
		return fmt.Errorf("persistent core directory must not contain symbolic links")
	}
	return nil
}

// publishBinary installs an immutable, content-addressed file. It never replaces
// the bundled binary, nor a file that a running core may still be executing.
func (u *coreUpdater) publishBinary(reader io.Reader, version string) (*coreArtifact, error) {
	f, err := os.CreateTemp(u.dir, ".core-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(reader, maxBinaryBytes+1))
	if err != nil {
		return nil, err
	}
	if n == 0 || n > maxBinaryBytes {
		return nil, fmt.Errorf("core binary exceeds size limit or is empty")
	}
	if err := f.Chmod(0700); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	a := &coreArtifact{version, hex.EncodeToString(h.Sum(nil))}
	if _, err := os.Lstat(u.artifactPath(a)); err == nil {
		return a, u.verifyArtifact(a)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Rename(f.Name(), u.artifactPath(a)); err != nil {
		return nil, err
	}
	return a, nil
}
func (u *coreUpdater) loadSelection() error {
	raw, err := os.ReadFile(filepath.Join(u.dir, "selection.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := u.prepareDirectory(); err != nil {
		return err
	}
	var state coreSelection
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if state.Schema != 1 {
		return fmt.Errorf("unsupported core selection schema")
	}
	if err := u.verifyArtifact(state.Active); err != nil {
		return err
	}
	// A damaged rollback copy must not prevent a valid active core from starting.
	u.selection = state
	return nil
}
func (u *coreUpdater) saveSelection(state coreSelection) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(u.dir, ".selection-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(raw); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Until this atomic commit, a panel restart selects the previous known-good
	// binary. Candidates are never persisted before successful runtime checks.
	if err := os.Rename(f.Name(), filepath.Join(u.dir, "selection.json")); err != nil {
		return err
	}
	if dir, err := os.Open(u.dir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
func (u *coreUpdater) pruneBinaries(activePath string) {
	keep := map[string]bool{}
	if filepath.Dir(activePath) == u.dir {
		keep[filepath.Base(activePath)] = true
	}
	for _, a := range []*coreArtifact{u.selection.Active, u.selection.Previous} {
		if a != nil {
			keep["mihomo-"+a.SHA256] = true
		}
	}
	entries, _ := os.ReadDir(u.dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "mihomo-") && len(name) == len("mihomo-")+64 && sha256Pattern.MatchString(name[len("mihomo-"):]) && !keep[name] {
			_ = os.Remove(filepath.Join(u.dir, name))
		}
	}
}

// saveJob persists one bounded job record so a browser or panel restart can
// explain an interrupted update. It contains no config or credentials.
func (u *coreUpdater) saveJob() error {
	raw, err := json.Marshal(u.job)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(u.dir, ".job-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(u.dir, "job.json"))
}
