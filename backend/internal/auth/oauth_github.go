package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const githubOAuthSettingKey = "github_oauth"

// GithubOAuthSettings is stored as JSON in panel_settings.
type GithubOAuthSettings struct {
	Enabled       bool     `json:"enabled"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	AllowedLogins []string `json:"allowed_logins"` // GitHub usernames (case-insensitive); empty = only already-linked accounts
}

type githubOAuthPublic struct {
	Enabled  bool   `json:"enabled"`
	ClientID string `json:"client_id,omitempty"`
}

type oauthStateEntry struct {
	Created time.Time
}

var (
	oauthStates   = map[string]oauthStateEntry{}
	oauthStatesMu sync.Mutex
)

func loadGithubOAuthSettings(db *gorm.DB) (GithubOAuthSettings, error) {
	var row models.PanelSetting
	var s GithubOAuthSettings
	if err := db.Where("key = ?", githubOAuthSettingKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s, nil
		}
		return s, err
	}
	if row.Value == "" {
		return s, nil
	}
	_ = json.Unmarshal([]byte(row.Value), &s)
	return s, nil
}

func saveGithubOAuthSettings(db *gorm.DB, s GithubOAuthSettings) error {
	// Normalize allowed logins
	out := make([]string, 0, len(s.AllowedLogins))
	seen := map[string]struct{}{}
	for _, l := range s.AllowedLogins {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	s.AllowedLogins = out
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var row models.PanelSetting
	err = db.Where("key = ?", githubOAuthSettingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: githubOAuthSettingKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&row).Update("value", string(raw)).Error
}

func publicGithubOAuth(s GithubOAuthSettings) githubOAuthPublic {
	if !s.Enabled || strings.TrimSpace(s.ClientID) == "" {
		return githubOAuthPublic{Enabled: false}
	}
	return githubOAuthPublic{Enabled: true, ClientID: strings.TrimSpace(s.ClientID)}
}

func newOAuthState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	oauthStatesMu.Lock()
	// prune old
	now := time.Now()
	for k, v := range oauthStates {
		if now.Sub(v.Created) > 15*time.Minute {
			delete(oauthStates, k)
		}
	}
	oauthStates[state] = oauthStateEntry{Created: now}
	oauthStatesMu.Unlock()
	return state, nil
}

func consumeOAuthState(state string) bool {
	oauthStatesMu.Lock()
	defer oauthStatesMu.Unlock()
	e, ok := oauthStates[state]
	if !ok {
		return false
	}
	delete(oauthStates, state)
	return time.Since(e.Created) <= 15*time.Minute
}

func resolveOAuthRedirectBase(cHost, publicURL string) string {
	pub := strings.TrimSpace(publicURL)
	if pub != "" {
		return strings.TrimRight(pub, "/")
	}
	// Fallback: derive from request (may be wrong behind broken reverse proxies).
	return strings.TrimRight(cHost, "/")
}

func githubAuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "read:user")
	q.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

type githubTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func exchangeGithubCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr githubTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tr.Error != "" {
		return "", fmt.Errorf("github token: %s (%s)", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", errors.New("github returned empty access_token")
	}
	return tr.AccessToken, nil
}

func fetchGithubUser(ctx context.Context, accessToken string) (*githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "3m-ui-oauth")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github user api: %s", strings.TrimSpace(string(body)))
	}
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	if u.ID == 0 || u.Login == "" {
		return nil, errors.New("github user missing id/login")
	}
	return &u, nil
}

func loginOrLinkGithubUser(db *gorm.DB, jwtSecret string, gh *githubUser, allowed []string) (*LoginResult, error) {
	ghID := fmt.Sprintf("%d", gh.ID)
	loginLower := strings.ToLower(strings.TrimSpace(gh.Login))

	var user models.User
	err := db.Where("github_id = ?", ghID).First(&user).Error
	if err == nil {
		return issueSessionForUser(db, jwtSecret, &user)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Not linked yet: require allowlist (or match username).
	allowedOK := false
	for _, a := range allowed {
		if strings.ToLower(strings.TrimSpace(a)) == loginLower {
			allowedOK = true
			break
		}
	}
	if !allowedOK {
		return nil, errors.New("github account not allowed; add the username to allowed_logins in Settings, or link an account first")
	}

	// Prefer existing user with same username as GitHub login.
	err = db.Where("LOWER(username) = ?", loginLower).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Fall back to the sole administrator.
		var admins []models.User
		if qerr := db.Where("role = ?", "admin").Limit(2).Find(&admins).Error; qerr != nil {
			return nil, qerr
		}
		if len(admins) != 1 {
			return nil, errors.New("no local user to bind; create an admin with the same username as GitHub, or keep a single admin account")
		}
		user = admins[0]
	} else if err != nil {
		return nil, err
	}

	// Bind GitHub identity.
	if err := db.Model(&user).Updates(map[string]interface{}{
		"github_id":    ghID,
		"github_login": gh.Login,
	}).Error; err != nil {
		return nil, err
	}
	user.GithubID = ghID
	user.GithubLogin = gh.Login
	return issueSessionForUser(db, jwtSecret, &user)
}

func issueSessionForUser(db *gorm.DB, jwtSecret string, user *models.User) (*LoginResult, error) {
	if user.SessionVersion == 0 {
		user.SessionVersion = 1
		if err := db.Model(user).Update("session_version", user.SessionVersion).Error; err != nil {
			return nil, err
		}
	}
	token, exp, err := GenerateToken(jwtSecret, user.ID, user.Username, user.Role, user.SessionVersion, DefaultTokenTTL)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:              token,
		ExpiresAt:          exp,
		Username:           user.Username,
		Role:               user.Role,
		MustChangePassword: user.MustChangePassword,
	}, nil
}
