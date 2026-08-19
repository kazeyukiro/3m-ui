package node_test

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
)

func TestValidateNode(t *testing.T) {
	// Valid shadowsocks node
	n1 := models.Listener{
		Name:        "hk-ss",
		Protocol:    "shadowsocks",
		Port:        "8388",
		BindAddress: "0.0.0.0",
		Config:      `{"password": "mypassword"}`,
	}
	if err := node.ValidateNode(&n1); err != nil {
		t.Fatalf("expected valid ss node to pass, got %v", err)
	}

	// Authentication is managed by ProxyUser, so an empty node-level
	// credential config is valid.
	n2 := models.Listener{
		Name:        "hk-ss",
		Protocol:    "shadowsocks",
		Port:        "8388",
		BindAddress: "0.0.0.0",
		Config:      `{}`,
	}
	if err := node.ValidateNode(&n2); err != nil {
		t.Fatalf("expected node without embedded credentials to pass, got %v", err)
	}

	// Invalid Port
	n3 := models.Listener{
		Name:        "hk-ss",
		Protocol:    "shadowsocks",
		Port:        "99999",
		BindAddress: "0.0.0.0",
		Config:      `{"password": "pass"}`,
	}
	if err := node.ValidateNode(&n3); err == nil {
		t.Fatal("expected node with invalid port to fail validation")
	}
}

func TestGenerateConfigYAML(t *testing.T) {
	dbNodes := []models.Listener{
		{
			Name:        "hk-ss",
			Protocol:    "shadowsocks",
			Port:        "8388",
			BindAddress: "0.0.0.0",
			Enabled:     true,
			Config:      `{"password": "secretpwd"}`,
		},
		{
			Name:        "hk-vmess",
			Protocol:    "vmess",
			Port:        "10086",
			BindAddress: "127.0.0.1",
			Enabled:     true,
			// Official VMess listener schema requires users as a list of
			// {username, uuid, alterId?} objects — not a top-level uuid field.
			Config: `{"users":[{"username":"u1","uuid":"test-uuid-1234","alterId":0}]}`,
		},
	}

	yamlStr, err := node.GenerateConfigYAML(dbNodes)
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	if !strings.Contains(yamlStr, "hk-ss") {
		t.Fatal("expected yaml to contain shadowsocks node hk-ss")
	}

	if !strings.Contains(yamlStr, "password: secretpwd") {
		t.Fatal("expected yaml to contain SS password")
	}

	if !strings.Contains(yamlStr, "uuid: test-uuid-1234") {
		t.Fatal("expected yaml to contain VMess UUID")
	}
	if !strings.Contains(yamlStr, "port: 10086") {
		t.Fatal("expected numeric port for single-port listener")
	}
}
