package converter

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

var SubconverterURL = "http://127.0.0.1:25500"

const maxSubconverterResponseBytes = 16 << 20

// CallSubconverter calls the local subconverter service to convert standard configurations.
func CallSubconverter(cfg *config.Config, token string, target string, rawYAML []byte) ([]byte, error) {
	return CallSubconverterWithRequest(cfg, token, target, rawYAML)
}

// CallSubconverterWithRequest is the request-independent implementation used
// by both the public subscription route and internal callers.
func CallSubconverterWithRequest(cfg *config.Config, token string, target string, rawYAML []byte) ([]byte, error) {
	port := 8080
	if cfg != nil && cfg.Server.Port != 0 {
		port = cfg.Server.Port
	}

	sourceURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/client/sub/%s?raw=true", port, token)

	u, err := url.Parse(fmt.Sprintf("%s/sub", SubconverterURL))
	if err != nil {
		return nil, fmt.Errorf("invalid subconverter URL: %w", err)
	}

	q := u.Query()
	q.Set("target", target)
	q.Set("url", sourceURL)
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("subconverter service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSubconverterResponseBytes))
		return nil, fmt.Errorf("subconverter returned error status %d: %s", resp.StatusCode, string(body))
	}

	converted, err := io.ReadAll(io.LimitReader(resp.Body, maxSubconverterResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read subconverter response: %w", err)
	}
	if int64(len(converted)) > maxSubconverterResponseBytes {
		return nil, fmt.Errorf("subconverter response exceeds 16 MiB limit")
	}

	return converted, nil
}
