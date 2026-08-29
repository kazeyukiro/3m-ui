package app

import (
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Container struct {
	DB                *gorm.DB
	Config            *config.Config
	Mihomo            *mihomo.Service
	Listener          *listener.Service
	Node              *node.Service
	User              *user.Service
	Traffic           *traffic.Service
	TrafficCollector  *traffic.Collector
	TrafficScheduler  *traffic.Scheduler
	System            *system.Service
	ConfigEngine      *dbconfig.ConfigEngine
	TelegramBot       *telegram.Bot
	TelegramScheduler *telegram.Scheduler
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {
	mihomoSvc := mihomo.NewService(cfg)
	// Wire crash notification: when the mihomo process exits unexpectedly
	// (non-zero exit while desired==true), emit a Telegram alert.
	mihomoSvc.SetCrashHandler(func(exitErr error) {
		telegram.NotifyCrash(db, exitErr)
	})
	listenerSvc := listener.NewService(db, cfg.Mihomo.Config, mihomoSvc)
	nodeSvc := node.NewService(db, cfg.Mihomo.Config, mihomoSvc)
	nodeSvc.SetRegenerator(listenerSvc)
	userSvc := user.NewService(db)
	systemSvc := system.NewService()
	userSvc.SetCredentialsChangedHandler(func() error {
		return nodeSvc.RegenerateConfig()
	})

	trafficSvc := traffic.NewService()
	userTraffic := traffic.NewUserService(db)
	collector := traffic.NewCollectorFromDefaults(db, trafficSvc, userTraffic)
	enforcer := traffic.NewEnforcer(db, nodeSvc)
	scheduler := traffic.NewScheduler(collector, enforcer, 0)
	scheduler.SetNotifier(telegram.NewNotifier(db))
	scheduler.Start()

	tgBot := telegram.NewBot(db, mihomoSvc, userSvc)
	tgBot.Start()

	tgScheduler := telegram.NewScheduler(db, mihomoSvc, userSvc, systemSvc, cfg.Database.Path, cfg.Mihomo.Config)
	tgScheduler.Start()

	return &Container{
		DB:                db,
		Config:            cfg,
		Mihomo:            mihomoSvc,
		Listener:          listenerSvc,
		Node:              nodeSvc,
		User:              userSvc,
		Traffic:           trafficSvc,
		TrafficCollector:  collector,
		TrafficScheduler:  scheduler,
		System:            systemSvc,
		ConfigEngine:      dbconfig.NewConfigEngine(db),
		TelegramBot:       tgBot,
		TelegramScheduler: tgScheduler,
	}
}

func (c *Container) RouterDeps() router.Deps {
	return router.Deps{
		DB:               c.DB,
		Config:           c.Config,
		Mihomo:           c.Mihomo,
		Traffic:          c.Traffic,
		TrafficCollector: c.TrafficCollector,
		User:             c.User,
		Node:             c.Node,
		Listener:         c.Listener,
		System:           c.System,
	}
}
