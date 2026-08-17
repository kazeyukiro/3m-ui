package models

import (
	"encoding/json"
	"testing"
)

func TestListenerJSONUsesLowercaseID(t *testing.T) {
	listener := Listener{BaseModel: BaseModel{ID: 42}, Name: "test-listener"}

	data, err := json.Marshal(listener)
	if err != nil {
		t.Fatalf("marshal listener: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal listener JSON: %v", err)
	}

	if got, ok := payload["id"].(float64); !ok || got != 42 {
		t.Fatalf("expected JSON id=42, got %#v", payload["id"])
	}
	if _, ok := payload["ID"]; ok {
		t.Fatalf("listener JSON must not expose an uppercase ID field")
	}
}
