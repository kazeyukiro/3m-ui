package router

import (
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Deps holds runtime dependencies for HTTP handlers.
type Deps struct {
	DB               *gorm.DB
	Config           *config.Config
	Mihomo           *mihomo.Service
	Traffic          *traffic.Service
	TrafficCollector *traffic.Collector
	User             *user.Service
	Node             *node.Service
	Listener         *listener.Service
	System           *system.Service
}

func (d Deps) mihomoService() *mihomo.Service       { return d.Mihomo }
func (d Deps) trafficService() *traffic.Service     { return d.Traffic }
func (d Deps) trafficCollector() *traffic.Collector { return d.TrafficCollector }
func (d Deps) userService() *user.Service           { return d.User }
func (d Deps) nodeService() *node.Service           { return d.Node }
func (d Deps) listenerService() *listener.Service   { return d.Listener }
func (d Deps) systemService() *system.Service       { return d.System }

func resolveDB(d Deps) *gorm.DB {
	return d.DB
}
