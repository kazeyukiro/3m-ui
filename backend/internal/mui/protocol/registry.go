package protocol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
)

type Share struct {
	URI        string
	QRContent  string
	ClientYAML []byte
}

type Module interface {
	Kind() domain.ProtocolKind
	Compile(context.Context, domain.Node, time.Time) (any, error)
	BuildShare(domain.DesiredState, domain.Node, domain.NodeUser, domain.AccessProfile) (Share, error)
	Capability() ProtocolCapability
}

type Registry struct {
	modules map[domain.ProtocolKind]Module
}

func DefaultRegistry() Registry {
	return NewRegistry(
		VLESSModule{}, Hysteria2Module{}, VMessModule{}, TrojanModule{}, ShadowsocksModule{},
	)
}

func NewRegistry(modules ...Module) Registry {
	registry := Registry{modules: make(map[domain.ProtocolKind]Module, len(modules))}
	for _, module := range modules {
		if module == nil {
			continue
		}
		registry.modules[module.Kind()] = module
	}
	return registry
}

func (registry Registry) Compile(ctx context.Context, node domain.Node, asOf time.Time) (any, error) {
	module, exists := registry.modules[node.Protocol]
	if !exists {
		return nil, fmt.Errorf("unsupported node protocol %q", node.Protocol)
	}
	compiled, err := module.Compile(ctx, node, asOf)
	if err != nil {
		return nil, fmt.Errorf("compile %s node %q: %w", node.Protocol, node.Name, err)
	}
	return compiled, nil
}

func (registry Registry) BuildShare(
	state domain.DesiredState,
	node domain.Node,
	user domain.NodeUser,
	profile domain.AccessProfile,
) (Share, error) {
	module, exists := registry.modules[node.Protocol]
	if !exists {
		return Share{}, fmt.Errorf("unsupported node protocol %q", node.Protocol)
	}
	if user.NodeID != node.ID || profile.NodeID != node.ID {
		return Share{}, errors.New("share input does not belong to the requested node")
	}
	if !node.Enabled {
		return Share{}, errors.New("disabled node cannot be shared")
	}
	if !user.Enabled || (user.ExpiresAt != nil && !user.ExpiresAt.After(state.AsOf)) {
		return Share{}, errors.New("disabled or expired user cannot be shared")
	}
	return module.BuildShare(state, node, user, profile)
}

func effectiveUsers(node domain.Node, asOf time.Time) []domain.NodeUser {
	return node.EffectiveUsers(asOf)
}
