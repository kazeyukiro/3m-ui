package protocol

import (
	"encoding/json"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
)

// ProtocolCapability is retained for Module interface parity with m-ui.
// Full schema manifests can be expanded later; Compile/BuildShare do not depend on it.
type ProtocolCapability struct {
	Kind        domain.ProtocolKind `json:"kind"`
	Label       string              `json:"label"`
	DefaultNode json.RawMessage     `json:"default_node,omitempty"`
	DefaultUser json.RawMessage     `json:"default_user,omitempty"`
	Features    []string            `json:"features,omitempty"`
}
