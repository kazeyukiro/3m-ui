package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

type Service struct {
	db         *gorm.DB
	httpClient *http.Client
}

func NewService(db *gorm.DB) *Service {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: safeClusterDialContext(dialer),
	}
	return &Service{
		db: db,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				if req.URL == nil || req.URL.Hostname() == "" {
					return fmt.Errorf("redirect target has no host")
				}
				if err := assertClusterHostAllowed(req.URL.Hostname()); err != nil {
					return fmt.Errorf("redirect target blocked: %w", err)
				}
				return nil
			},
		},
	}
}

// safeClusterDialContext binds SSRF validation to the actual TCP connection.
// Hostname validation alone is vulnerable to DNS rebinding between validation
// and connect time, so every resolved address is checked immediately before
// dialing. Private/link-local/metadata addresses are never dialed unless the
// explicit lab override is enabled; cloud metadata endpoints remain blocked.
func safeClusterDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid dial address %q: %w", address, err)
		}
		if ip := net.ParseIP(host); ip != nil {
			if err := assertClusterIPAllowed(ip); err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}

		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", host, err)
		}
		var lastErr error
		for _, ip := range ips {
			if err := assertClusterIPAllowed(ip); err != nil {
				lastErr = err
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, fmt.Errorf("connect to %q: %w", host, lastErr)
		}
		return nil, fmt.Errorf("no allowed addresses for %q", host)
	}
}

type CreateInput struct {
	Name     string `json:"name" binding:"required"`
	BaseURL  string `json:"base_url" binding:"required"`
	APIToken string `json:"api_token"`
	Enabled  *bool  `json:"enabled"`
	Remark   string `json:"remark"`
}

type UpdateInput struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIToken  string `json:"api_token"`
	Enabled   *bool  `json:"enabled"`
	Remark    string `json:"remark"`
	KeepToken bool   `json:"keep_token"`
}

func (s *Service) List() ([]models.RemoteServer, error) {
	var rows []models.RemoteServer
	if err := s.db.Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].APITokenSet = rows[i].APIToken != ""
		rows[i].APIToken = ""
	}
	return rows, nil
}

func (s *Service) Create(in CreateInput) (*models.RemoteServer, error) {
	name := strings.TrimSpace(in.Name)
	base, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	row := &models.RemoteServer{
		Name:     name,
		BaseURL:  base,
		APIToken: strings.TrimSpace(in.APIToken),
		Enabled:  enabled,
		Remark:   strings.TrimSpace(in.Remark),
	}
	if err := s.db.Create(row).Error; err != nil {
		return nil, err
	}
	return sanitize(row), nil
}

func (s *Service) Update(id uint, in UpdateInput) (*models.RemoteServer, error) {
	var row models.RemoteServer
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		row.Name = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.BaseURL) != "" {
		base, err := normalizeBaseURL(in.BaseURL)
		if err != nil {
			return nil, err
		}
		row.BaseURL = base
	}
	if !in.KeepToken && strings.TrimSpace(in.APIToken) != "" {
		row.APIToken = strings.TrimSpace(in.APIToken)
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	row.Remark = strings.TrimSpace(in.Remark)
	if err := s.db.Save(&row).Error; err != nil {
		return nil, err
	}
	return sanitize(&row), nil
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&models.RemoteServer{}, id).Error
}

func (s *Service) HealthCheck(id uint) (*models.RemoteServer, error) {
	var row models.RemoteServer
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	return s.runHealth(&row)
}

func (s *Service) HealthCheckAll() ([]models.RemoteServer, error) {
	var rows []models.RemoteServer
	if err := s.db.Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]models.RemoteServer, 0, len(rows))
	for i := range rows {
		r, err := s.runHealth(&rows[i])
		if err != nil {
			continue
		}
		out = append(out, *r)
	}
	var disabled []models.RemoteServer
	_ = s.db.Where("enabled = ?", false).Find(&disabled).Error
	for i := range disabled {
		out = append(out, *sanitize(&disabled[i]))
	}
	return out, nil
}

func (s *Service) runHealth(row *models.RemoteServer) (*models.RemoteServer, error) {
	target := strings.TrimRight(row.BaseURL, "/") + "/api/v1/health"
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if row.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+row.APIToken)
	}
	now := time.Now().UTC()
	row.LastCheckAt = &now
	resp, err := s.httpClient.Do(req)
	if err != nil {
		row.LastStatus = "down"
		row.LastError = truncateErr(err.Error())
		_ = s.db.Save(row).Error
		return sanitize(row), nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		row.LastStatus = "up"
		row.LastError = ""
	} else {
		row.LastStatus = "error"
		row.LastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	if saveErr := s.db.Save(row).Error; saveErr != nil {
		return nil, fmt.Errorf("health check result could not be saved: %w", saveErr)
	}
	return sanitize(row), nil
}

func (s *Service) FetchRemoteNodes(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodGet, "/api/v1/nodes", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	return json.RawMessage(raw), nil
}

func (s *Service) FetchDashboard(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodGet, "/api/v1/dashboard", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	return json.RawMessage(raw), nil
}

func (s *Service) FetchUsers(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodGet, "/api/v1/users", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	return json.RawMessage(raw), nil
}

func (s *Service) RestartCore(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodPost, "/api/v1/mihomo/restart", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return json.RawMessage(`{"status":"ok"}`), nil
	}
	return json.RawMessage(raw), nil
}

func (s *Service) StartCore(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodPost, "/api/v1/mihomo/start", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return json.RawMessage(`{"status":"ok"}`), nil
	}
	return json.RawMessage(raw), nil
}

func (s *Service) StopCore(id uint) (json.RawMessage, error) {
	status, raw, err := s.ProxyRemote(id, http.MethodPost, "/api/v1/mihomo/stop", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return json.RawMessage(`{"status":"ok"}`), nil
	}
	return json.RawMessage(raw), nil
}

func (s *Service) ProxyRemote(id uint, method, path string, body []byte) (int, []byte, error) {
	var row models.RemoteServer
	if err := s.db.First(&row, id).Error; err != nil {
		return 0, nil, err
	}
	if !row.Enabled {
		return 0, nil, fmt.Errorf("remote server is disabled")
	}
	path, err := sanitizeProxyPath(path)
	if err != nil {
		return 0, nil, err
	}
	full := strings.TrimRight(row.BaseURL, "/") + path
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, full, rdr)
	if err != nil {
		return 0, nil, err
	}
	if row.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+row.APIToken)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

func sanitize(row *models.RemoteServer) *models.RemoteServer {
	if row == nil {
		return nil
	}
	row.APITokenSet = row.APIToken != ""
	row.APIToken = ""
	return row
}

func truncateErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("base_url is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("base_url must use http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("base_url host is required")
	}
	if u.User != nil {
		return "", fmt.Errorf("base_url userinfo is not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("base_url host is required")
	}
	if err := assertClusterHostAllowed(host); err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

var allowedPrefixes = []string{
	"/api/v1/health",
	"/api/v1/dashboard",
	"/api/v1/nodes",
	"/api/v1/listeners",
	"/api/v1/users",
	"/api/v1/mihomo",
	"/api/v1/system",
	"/api/v1/traffic",
}

func sanitizeProxyPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.Contains(path, "..") || strings.Contains(path, "://") || strings.ContainsAny(path, " \t\r\n") {
		return "", fmt.Errorf("invalid path")
	}
	pathOnly := path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		pathOnly = path[:i]
	}
	ok := false
	for _, p := range allowedPrefixes {
		if pathOnly == p || strings.HasPrefix(pathOnly, p+"/") {
			ok = true
			break
		}
	}
	if !ok {
		return "", fmt.Errorf("path not allowed for remote proxy")
	}
	return path, nil
}
