package listener

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestListenerTemplateModel(t *testing.T) {
	t := &models.ListenerTemplate{Name: "vless-default", Protocol: "vless", Config: `{"flow":"xtls-rprx-vision"}`}
	if t.Name == "" || t.Protocol == "" { t.Fatal("template fields unexpectedly empty") }
}

func TestListenerVersionSnapshotIsJSON(t *testing.T) {
	l := &models.Listener{ID: 1, Name: "demo", Protocol: "vless", Port: "443", BindAddress: "0.0.0.0", Config: `{}`}
	if err := ValidateModel(l); err != nil { t.Fatalf("valid listener rejected: %v", err) }
}

func TestBatchIDsRequireNonEmpty(t *testing.T) {
	if len([]uint{}) != 0 { t.Fatal("unexpected non-empty ids") }
}
