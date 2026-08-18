package config

import "testing"

func TestIsMihomoListenerProtocol(t *testing.T) {
	supported := []string{
		"shadowsocks", "snell", "vmess", "vless", "trojan", "hysteria2", "hysteria2-realm",
		"tuic", "shadowquic", "anytls", "mieru", "sudoku", "trusttunnel",
	}
	for _, protocol := range supported {
		if !IsMihomoListenerProtocol(protocol) {
			t.Errorf("expected %q to be supported", protocol)
		}
	}
}

func TestIsMihomoListenerProtocolRejectsInboundOnlyTypes(t *testing.T) {
	rejected := []string{"socks", "http", "tproxy", "redir", "mixed", "tunnel", "tun", "wireguard", ""}
	for _, protocol := range rejected {
		if IsMihomoListenerProtocol(protocol) {
			t.Errorf("expected %q to be rejected", protocol)
		}
	}
}

func TestValidateRejectsDefaultSecrets(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: "test.db"},
		JWT: JWTConfig{Secret: DefaultJWTSecret},
		Security: SecurityConfig{CredentialKey: "a-unique-credential-key-that-is-long-enough"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected default JWT secret to be rejected")
	}

	cfg.JWT.Secret = "a-unique-jwt-secret-that-is-long-enough"
	cfg.Security.CredentialKey = DefaultCredentialKey
	if err := Validate(cfg); err == nil {
		t.Fatal("expected default credential key to be rejected")
	}
}

func TestValidateAcceptsSecureConfig(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: "test.db"},
		JWT:      JWTConfig{Secret: "a-unique-jwt-secret-that-is-long-enough"},
		Security: SecurityConfig{CredentialKey: "a-unique-credential-key-that-is-long-enough"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected secure config to pass validation: %v", err)
	}
}
