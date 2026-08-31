package traffic

import (
	"log"
	"sync"
	"time"
)

const DefaultInterval = 10 * time.Second

type Scheduler struct {
	collector *Collector
	enforcer  *Enforcer
	notifier  interface{ CheckAndNotify() }
	interval  time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup

	// startOnce guards Start() so the goroutine is launched exactly once even
	// if Start() is invoked multiple times (defensive against caller bugs in
	// container wiring / tests). Stop() stays a one-shot via stopCh.
	startOnce sync.Once
}

func NewScheduler(collector *Collector, enforcer *Enforcer, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Scheduler{
		collector: collector,
		enforcer:  enforcer,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (s *Scheduler) SetNotifier(n interface{ CheckAndNotify() }) {
	if s != nil {
		s.notifier = n
	}
}

func (s *Scheduler) Start() {
	// Idempotent: subsequent calls are no-ops so accidental double-Start (e.g.
	// from container wiring or tests) does not spawn a second collector loop
	// racing on the same enforcer/collector state.
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			s.tick()
			for {
				select {
				case <-ticker.C:
					s.tick()
				case <-s.stopCh:
					return
				}
			}
		}()
	})
}

func (s *Scheduler) tick() {
	if err := s.collector.CollectOnce(); err != nil {
		log.Printf("traffic: collection skipped: %v", err)
		return
	}
	if s.collector != nil {
		s.collector.EnforceIPLimits()
		if s.collector.db != nil {
			MaybeResetMonthlyTraffic(s.collector.db)
		}
	}
	if s.enforcer == nil {
		return
	}
	if _, err := s.enforcer.CheckAndEnforce(); err != nil {
		log.Printf("traffic: enforcement check failed: %v", err)
	}
	if s.notifier != nil {
		s.notifier.CheckAndNotify()
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}
