package cluster

import (
	"fmt"
	"io"
	"net/http"
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
	return &Service{
		db: db,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
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
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if name == "" || base == "" {
		return nil, fmt.Errorf("name and base_url are required")
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
	row.APITokenSet = row.APIToken != ""
	row.APIToken = ""
	return row, nil
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
		row.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
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
	row.APITokenSet = row.APIToken != ""
	row.APIToken = ""
	return &row, nil
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&models.RemoteServer{}, id).Error
}

// HealthCheck probes the remote panel health endpoint.
func (s *Service) HealthCheck(id uint) (*models.RemoteServer, error) {
	var row models.RemoteServer
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	url := row.BaseURL + "/api/v1/health"
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		row.LastError = err.Error()
		_ = s.db.Save(&row)
		row.APITokenSet = row.APIToken != ""
		row.APIToken = ""
		return &row, nil
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
	_ = s.db.Save(&row)
	row.APITokenSet = row.APIToken != ""
	row.APIToken = ""
	return &row, nil
}
