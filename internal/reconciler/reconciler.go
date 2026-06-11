// Package reconciler runs OPORD's periodic background loops. It is a thin,
// generic scheduler: the actual work (drift detection, provider health checks,
// ...) lives in the orchestrator and is wired in here as a ScanFunc so this
// package stays dependency-light. Each Loop carries a name for clear logs.
package reconciler

import (
	"context"
	"log/slog"
	"time"
)

// ScanFunc performs one pass and reports counts: how many items it checked, how
// many it flagged (drifted / unhealthy / ...), and how many errored.
type ScanFunc func(ctx context.Context) (checked, flagged, errored int, err error)

// Loop calls a ScanFunc on a fixed interval until its context is cancelled.
type Loop struct {
	name     string
	scan     ScanFunc
	interval time.Duration
	log      *slog.Logger
}

// New builds a named periodic loop. A non-positive interval disables it.
func New(name string, scan ScanFunc, interval time.Duration, log *slog.Logger) *Loop {
	if log == nil {
		log = slog.Default()
	}
	if name == "" {
		name = "periodic loop"
	}
	return &Loop{name: name, scan: scan, interval: interval, log: log}
}

// Run blocks, scanning every interval, until ctx is done. Safe to call in a
// goroutine.
func (l *Loop) Run(ctx context.Context) {
	if l.interval <= 0 {
		l.log.Info(l.name+" disabled", "interval", l.interval.String())
		return
	}
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	l.log.Info(l.name+" started", "interval", l.interval.String())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checked, flagged, errored, err := l.scan(ctx)
			if err != nil {
				l.log.Error(l.name+" scan failed", "err", err)
				continue
			}
			if checked > 0 || flagged > 0 || errored > 0 {
				l.log.Info(l.name+" scan complete", "checked", checked, "flagged", flagged, "errored", errored)
			}
		}
	}
}
