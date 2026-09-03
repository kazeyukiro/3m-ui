package config

import "testing"

func TestMihomoListenerSchemaRegistry(t *testing.T) {
	for _, protocol := range MihomoListenerProtocols {
		schema, ok := GetMihomoListenerSchema(protocol)
		if !ok {
			t.Fatalf("protocol %q has no schema", protocol)
		}
		if schema.Protocol != protocol {
			t.Fatalf("schema protocol mismatch: got %q want %q", schema.Protocol, protocol)
		}
		if len(schema.Fields) == 0 {
			t.Fatalf("protocol %q has no registered fields", protocol)
		}
	}
}

func TestMihomoListenerSchemaRejectsNonListenerProtocols(t *testing.T) {
	for _, protocol := range []string{"socks", "http", "mixed", "redir", "tproxy", "tun", "tunnel"} {
		if _, ok := GetMihomoListenerSchema(protocol); ok {
			t.Fatalf("non-distributable protocol %q must not have a node schema", protocol)
		}
	}
}

func TestMihomoListenerSchemaIncludesFieldsUsedByForms(t *testing.T) {
	cases := map[string][]string{
		"shadowsocks": {"cipher", "password", "simple-obfs", "shadow-tls", "res-tls", "jls-config", "kcp-tun", "mux-option"},
		"snell":       {"psk", "version", "obfs-opts", "shadow-tls", "res-tls", "jls-config"},
		"vless":       {"users", "ws-path", "grpc-service-name", "xhttp-config", "reality-config"},
		"vmess":       {"users", "ws-path", "grpc-service-name", "mekya-config", "mkcp-config", "reality-config"},
		"trojan":      {"users", "ws-path", "grpc-service-name", "reality-config", "ss-option"},
		"hysteria2":   {"users", "obfs", "certificate", "private-key", "alpn"},
		"tuic":        {"users", "token", "certificate", "private-key", "congestion-controller"},
		"anytls":      {"users", "certificate", "private-key", "padding-scheme"},
	}
	for protocol, fields := range cases {
		schema, ok := GetMihomoListenerSchema(protocol)
		if !ok {
			t.Fatalf("protocol %q has no schema", protocol)
		}
		for _, field := range fields {
			if _, ok := schema.Fields[field]; !ok {
				t.Errorf("protocol %q is missing form field %q", protocol, field)
			}
		}
	}
}

// TestMihomoListenerSchemaIncludesClientMetadataFields (fix-G-schema) asserts
// the R-M3 + R-M4 schema-whitelist additions are present. These fields are
// CLIENT-only per wiki (proxies-transport mekya-opts / grpc-opts blocks), but
// the panel stores them on listener JSON as metadata for client YAML
// generation; the schema MUST whitelist them or the panel's HARD-reject
// validator (listener_validation.go) refuses listener JSON that carries them.
func TestMihomoListenerSchemaIncludesClientMetadataFields(t *testing.T) {
	// R-M4: 5 grpc-opts extended fields (top-level Fields on vmess/vless/trojan).
	grpcExtFields := []string{"grpc-user-agent", "ping-interval", "max-connections", "min-streams", "max-streams"}
	for _, protocol := range []string{"vmess", "vless", "trojan"} {
		schema, ok := GetMihomoListenerSchema(protocol)
		if !ok {
			t.Fatalf("protocol %q has no schema", protocol)
		}
		for _, field := range grpcExtFields {
			if _, ok := schema.Fields[field]; !ok {
				t.Errorf("protocol %q is missing R-M4 client-metadata field %q", protocol, field)
			}
		}
	}

	// R-M3: 2 mekya-opts extended fields (NestedFields under mekya-config on vmess).
	schema, ok := GetMihomoListenerSchema("vmess")
	if !ok {
		t.Fatalf("protocol %q has no schema", "vmess")
	}
	parent, ok := schema.NestedFields["mekya-config"]
	if !ok {
		t.Fatalf("vmess schema missing mekya-config NestedFields parent")
	}
	for _, field := range []string{"polling-interval-initial", "h2-pool-size"} {
		if _, ok := parent[field]; !ok {
			t.Errorf("vmess mekya-config NestedFields missing R-M3 client-metadata field %q", field)
		}
	}
}
