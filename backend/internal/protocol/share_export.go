package protocol

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// ExportShares decodes a listener into the typed NodeModel and builds m-ui style shares.
func ExportShares(l models.Listener, publicHost string, users []UserCred) ([]Share, error) {
	node, err := DecodeNodeModel(l, users)
	if err != nil {
		return nil, err
	}
	if h := strings.TrimSpace(publicHost); h != "" {
		node.PublicHost = h
	}
	return DefaultCompileRegistry().BuildShares(node)
}

// ExportShareURIs returns only URI strings (compatibility with older callers).
func ExportShareURIs(l models.Listener, publicHost string, users []UserCred) ([]string, error) {
	shares, err := ExportShares(l, publicHost, users)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(shares))
	for _, s := range shares {
		if s.URI != "" {
			out = append(out, s.URI)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no share URIs generated")
	}
	return out, nil
}
