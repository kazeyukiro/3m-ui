package protocol

import "testing"

func TestVLESSCompilerMovesTopLevelFlowIntoUsers(t *testing.T) {
	result, err := (VLESSCompiler{}).Compile(CompileInput{
		Name:     "vless-reality",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config: map[string]interface{}{
			"flow": "xtls-rprx-vision",
			"reality-config": map[string]interface{}{
				"dest": "example:443",
			},
		},
		Users: []UserCred{{UUID: "11111111-1111-4111-8111-111111111111"}},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if _, ok := result["flow"]; ok {
		t.Fatal("top-level VLESS flow must not be emitted")
	}
	users, ok := result["users"].([]map[string]interface{})
	if !ok || len(users) != 1 {
		t.Fatalf("unexpected users: %#v", result["users"])
	}
	if users[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("expected user flow, got %#v", users[0]["flow"])
	}
}

func TestVLESSCompilerPreservesPerUserFlow(t *testing.T) {
	result, err := (VLESSCompiler{}).Compile(CompileInput{
		Name:     "vless",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config: map[string]interface{}{
			"flow": "xtls-rprx-vision",
		},
		Users: []UserCred{{UUID: "11111111-1111-4111-8111-111111111111", Flow: ""}},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	users := result["users"].([]map[string]interface{})
	if users[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("expected fallback flow, got %#v", users[0]["flow"])
	}
}

func TestVLESSCompilerDoesNotOverrideExplicitUserFlow(t *testing.T) {
	result, err := (VLESSCompiler{}).Compile(CompileInput{
		Name:     "vless",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config: map[string]interface{}{
			"flow": "xtls-rprx-vision",
		},
		Users: []UserCred{{UUID: "11111111-1111-4111-8111-111111111111", Flow: "custom-flow"}},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	users := result["users"].([]map[string]interface{})
	if users[0]["flow"] != "custom-flow" {
		t.Fatalf("explicit user flow was overwritten: %#v", users[0]["flow"])
	}
}

func TestVLESSCompilerNeverUsesPasswordAsUUID(t *testing.T) {
	result, err := (VLESSCompiler{}).Compile(CompileInput{
		Name:     "vless",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Users:    []UserCred{{Password: "not-a-uuid"}},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if _, ok := result["users"]; ok {
		t.Fatalf("VLESS credential without UUID must not be emitted as a UUID: %#v", result["users"])
	}
}
