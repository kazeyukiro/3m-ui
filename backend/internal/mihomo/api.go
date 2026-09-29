package mihomo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ExternalControllerAPI wraps communication with the Mihomo External Controller REST API
type ExternalControllerAPI struct {
	BaseURL string
	Secret  string
	client  *http.Client
}

func NewExternalControllerAPI(baseURL, secret string) *ExternalControllerAPI {
	return &ExternalControllerAPI{
		BaseURL: baseURL,
		Secret:  secret,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetConfig fetches the current configuration from Mihomo's external API
func (api *ExternalControllerAPI) GetConfig() (string, error) {
	req, err := http.NewRequest("GET", api.BaseURL+"/configs", nil)
	if err != nil {
		return "", err
	}

	if api.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+api.Secret)
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mihomo core API offline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	bytesData, _ := json.MarshalIndent(data, "", "  ")
	return string(bytesData), nil
}

// ReloadConfig sends a PUT request to trigger config reload in Mihomo Core
func (api *ExternalControllerAPI) ReloadConfig(payload map[string]interface{}) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", api.BaseURL+"/configs?force=true", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if api.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+api.Secret)
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return fmt.Errorf("mihomo core API offline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to reload, status: %s", resp.Status)
	}

	return nil
}

// ListRuleProviders GET /providers/rules
func (api *ExternalControllerAPI) ListRuleProviders() (map[string]interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, api.BaseURL+"/providers/rules", nil)
	if err != nil {
		return nil, err
	}
	if api.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+api.Secret)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mihomo core API offline: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list rule providers: status %s", resp.Status)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// UpdateRuleProvider PUT /providers/rules/{name} — hot-reload one rule-set (official Clash API).
func (api *ExternalControllerAPI) UpdateRuleProvider(name string) error {
	if name == "" {
		return fmt.Errorf("rule provider name is empty")
	}
	req, err := http.NewRequest(http.MethodPut, api.BaseURL+"/providers/rules/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	if api.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+api.Secret)
	}
	// Rule-set download can be slow
	client := *api.client
	client.Timeout = 120 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("mihomo core API offline: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("update rule provider %q: status %s", name, resp.Status)
	}
	return nil
}
