package app

import (
	"log"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/healthcheck"
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
	HealthMonitor     *healthcheck.Monitor
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
		return mihomoSvc.SyncCredentials(nodeSvc.RegenerateConfig)
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

	// Tunnel health watchdog. Opt-in: it only probes/restarts when persisted
	// settings have Enabled=true. The adapter uses the mihomo.Service API so
	// all existing restart guards (core-update busy, config validation,
	// lifecycle serialization) are honoured.
	healthCore := &healthCoreAdapter{svc: mihomoSvc}
	healthMonitor := healthcheck.NewMonitor(nil, healthCore)
	if saved, err := healthcheck.LoadSettings(db); err == nil {
		healthMonitor.Apply(saved)
	} else {
		log.Printf("[WARNING] load tunnel health settings failed: %v; monitor stays disabled", err)
	}
	healthMonitor.Start()

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
		HealthMonitor:     healthMonitor,
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
		HealthMonitor:    c.HealthMonitor,
	}
}

// Shutdown gracefully stops the background goroutines spawned by NewContainer
// (traffic scheduler, Telegram bot long-poll loop, Telegram scheduler). It is
// safe to call even when some components are nil. The HTTP server itself is
// not stopped here — callers should signal/interrupt the server first (or
// rely on the process exiting) and then invoke Shutdown to release goroutines.
//
// Note: this method is currently NOT wired to OS signals; it exists so that
// future signal-handling code (or tests) can call it. Each Start() is also
// idempotent via sync.Once, so calling Stop() on a never-started component is
// still well-defined for the traffic/telegram schedulers and bot.
func (c *Container) Shutdown() {
	if c == nil {
		return
	}
	if c.TrafficScheduler != nil {
		c.TrafficScheduler.Stop()
	}
	if c.TelegramBot != nil {
		c.TelegramBot.Stop()
	}
	if c.TelegramScheduler != nil {
		c.TelegramScheduler.Stop()
	}
	if c.HealthMonitor != nil {
		c.HealthMonitor.Stop()
	}
}
