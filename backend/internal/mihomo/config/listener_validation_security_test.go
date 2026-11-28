package config

import "testing"

func TestValidateListenerConfigRejectsUnknownNestedField(t *testing.T) {
	raw := `{"reality-config":{"private-key":"secret","unknown":"value"}}`
	if err := ValidateListenerConfig("vless", raw); err == nil {
		t.Fatal("expected unknown nested field to be rejected")
	}
}

func TestValidateListenerConfigRejectsNestedScalar(t *testing.T) {
	raw := `{"reality-config":"not-an-object"}`
	if err := ValidateListenerConfig("vless", raw); err == nil {
		t.Fatal("expected nested object type violation to be rejected")
	}
}

func TestValidateListenerConfigAcceptsDeepNestedField(t *testing.T) {
	raw := `{"reality-config":{"limit-fallback-upload":{"after-bytes":1024}}}`
	if err := ValidateListenerConfig("vless", raw); err != nil {
		t.Fatalf("expected valid deep nested field, got %v", err)
	}
}
