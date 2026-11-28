package system

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Default GeoIP / GeoSite sources used by Mihomo (MetaCubeX).
var defaultGeoFiles = []struct {
	Name string
	URL  string
}{
	{Name: "geoip.metadb", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geoip.metadb"},
	{Name: "geosite.dat", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geosite.dat"},
	{Name: "geoip.dat", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geoip.dat"},
}

// UpdateGeoFiles downloads the latest GeoIP/GeoSite databases into dir
// (typically the Mihomo working directory next to config.yaml). .
func UpdateGeoFiles(dir string) (map[string]string, error) {
	if dir == "" {
		return nil, fmt.Errorf("geofile directory is empty")
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	result := make(map[string]string, len(defaultGeoFiles))
	for _, f := range defaultGeoFiles {
		if err := downloadFile(client, f.URL, filepath.Join(dir, f.Name)); err != nil {
			result[f.Name] = "error: " + err.Error()
			continue
		}
		result[f.Name] = "ok"
	}
	return result, nil
}

func downloadFile(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".geo-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}
