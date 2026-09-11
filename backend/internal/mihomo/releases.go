package mihomo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/MetaCubeX/mihomo/releases"
const releaseDownload = "https://github.com/MetaCubeX/mihomo/releases/download/"
const maxArchiveBytes int64 = 64 << 20
const maxBinaryBytes int64 = 128 << 20

// Thirty releases include hundreds of platform assets each.
const maxReleaseMetadataBytes = 16 << 20

var stableVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var alphaVersion = regexp.MustCompile(`^alpha-[a-f0-9]{7,40}$`)
var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

const alphaReleaseTag = "Prerelease-Alpha"

// CoreRelease is deliberately limited to official, verifiable artifacts.
// The API accepts a version, never a URL or an executable path.
type CoreRelease struct {
	Version     string `json:"version"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
	Asset       string `json:"asset"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	Prerelease  bool   `json:"prerelease"`
}
type githubRelease struct {
	Tag         string `json:"tag_name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	} `json:"assets"`
}

func corePlatform() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("core updates support Linux amd64 and arm64")
	}
	switch runtime.GOARCH {
	case "amd64":
		return "linux-amd64-v1", nil
	case "arm64":
		return "linux-arm64", nil
	default:
		return "", fmt.Errorf("core updates support Linux amd64 and arm64")
	}
}
func officialRelease(r githubRelease, platform string) (CoreRelease, bool) {
	alpha := r.Tag == alphaReleaseTag && r.Prerelease
	if r.Draft || (!alpha && (r.Prerelease || !stableVersion.MatchString(r.Tag))) {
		return CoreRelease{}, false
	}
	prefix := "mihomo-" + platform + "-"
	var result CoreRelease
	for _, a := range r.Assets {
		if !strings.HasPrefix(a.Name, prefix) || !strings.HasSuffix(a.Name, ".gz") {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(a.Name, prefix), ".gz")
		if (alpha && !alphaVersion.MatchString(version)) || (!alpha && version != r.Tag) {
			continue
		}
		// Fail closed while a rolling release contains ambiguous builds.
		if result.Version != "" {
			return CoreRelease{}, false
		}
		sum := strings.TrimPrefix(a.Digest, "sha256:")
		if !strings.HasPrefix(a.Digest, "sha256:") || !sha256Pattern.MatchString(sum) || a.Size <= 0 || a.Size > maxArchiveBytes {
			return CoreRelease{}, false
		}
		result = CoreRelease{Version: version, PublishedAt: r.PublishedAt, URL: "https://github.com/MetaCubeX/mihomo/releases/tag/" + r.Tag, Asset: a.Name, SHA256: sum, Size: a.Size, Prerelease: alpha}
	}
	return result, result.Version != ""
}
func coreHTTPClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil || (host != "github.com" && host != "api.github.com" && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com") {
			return fmt.Errorf("unexpected redirect from the official release service")
		}
		return nil
	}}
}
func (u *coreUpdater) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "3m-ui-core-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("official release request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("official release service returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
func (u *coreUpdater) readReleaseJSON(ctx context.Context, url string, out any) error {
	resp, err := u.get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseMetadataBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxReleaseMetadataBytes {
		return fmt.Errorf("release metadata exceeds size limit")
	}
	return json.Unmarshal(raw, out)
}
func (s *Service) CoreReleases(ctx context.Context) ([]CoreRelease, error) {
	if s == nil {
		return nil, fmt.Errorf("core updates are unavailable")
	}
	u := s.updater
	if u == nil {
		return nil, fmt.Errorf("core updates are unavailable")
	}
	platform, err := corePlatform()
	if err != nil {
		return nil, err
	}
	u.releaseMu.Lock()
	defer u.releaseMu.Unlock()
	if time.Since(u.releasesAt) < time.Minute && u.releases != nil {
		return append([]CoreRelease{}, u.releases...), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var releases []githubRelease
	if err := u.readReleaseJSON(ctx, releaseAPI+"?per_page=30", &releases); err != nil {
		return nil, err
	}
	result := []CoreRelease{}
	for _, r := range releases {
		if release, ok := officialRelease(r, platform); ok {
			result = append(result, release)
		}
	}
	// Keep the default choice stable even though GitHub lists rolling alpha first.
	sort.SliceStable(result, func(i, j int) bool { return !result[i].Prerelease && result[j].Prerelease })
	u.releases, u.releasesAt = result, time.Now()
	return append([]CoreRelease{}, result...), nil
}
