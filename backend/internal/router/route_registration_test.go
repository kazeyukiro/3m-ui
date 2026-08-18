package router_test

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
)

func TestSetupRouterWithListenerAndNodeServices(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.JWT.Secret = "test-secret-for-router-registration"
	cfg.Database.Path = "/tmp/3m-ui-route-registration-test.db"
	cfg.Mihomo.Config = "/tmp/3m-ui-route-registration-test.yaml"

	listenerSvc := listener.NewService(nil, cfg.Mihomo.Config, nil)
	nodeSvc := node.NewService(nil, cfg.Mihomo.Config, nil)

	// This used to panic because both services registered the same CRUD paths
	// on /api/v1/nodes and /api/v1/listeners. The router must now register
	// shared CRUD once and only the node-specific routes from node.Service.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("router registration panicked: %v", r)
		}
	}()

	_ = router.SetupRouterWithDeps(router.Deps{
		Config:   cfg,
		Listener: listenerSvc,
		Node:     nodeSvc,
	})
}
