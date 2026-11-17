package app

import "github.com/kazeyukiro/3m-ui/backend/internal/mihomo"

// healthCoreAdapter adapts *mihomo.Service to the healthcheck.coreState
// interface. Keeping the adapter in app (rather than healthcheck) avoids a
// package cycle: healthcheck defines only the small interface it needs.
type healthCoreAdapter struct {
	svc *mihomo.Service
}

func (a *healthCoreAdapter) Running() bool {
	if a == nil || a.svc == nil {
		return false
	}
	st, err := a.svc.GetStatus()
	if err != nil || st == nil {
		return false
	}
	return st.Running
}

func (a *healthCoreAdapter) Restart() error {
	if a == nil || a.svc == nil {
		return nil
	}
	return a.svc.RestartMihomo()
}
