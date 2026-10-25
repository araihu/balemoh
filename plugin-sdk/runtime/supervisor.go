package runtime

import (
	"context"
	"log/slog"
	"time"
)

// Process is the independent child-health boundary. Source freshness is not health.
type Process interface {
	Close()
	Exited() bool
	CheckHealth(context.Context) error
}
type Event struct {
	State    string
	Restarts int
	Duration time.Duration
}

// Supervise owns one capability until cancellation. Publish must return promptly;
// per-connection configuration/status work belongs to its own cancellable worker.
func Supervise[T Process](ctx context.Context, start func(context.Context) (T, error), publish func(T), withdraw func(T), observe func(Event)) {
	var attempts []time.Time
	restarts := 0
	emit := func(state string, since time.Time) {
		event := Event{state, restarts, time.Since(since)}
		if observe != nil {
			observe(event)
		}
	}
	for {
		if ctx.Err() != nil {
			return
		}
		began := time.Now()
		emit("starting", began)
		child, err := start(ctx)
		if err == nil {
			publish(child)
			emit("ready", began)
			ready := time.Now()
			exit := time.NewTicker(time.Second)
			health := time.NewTicker(10 * time.Second)
			failures := 0
			running := true
			for running {
				select {
				case <-ctx.Done():
					running = false
				case <-exit.C:
					if child.Exited() {
						running = false
					}
				case <-health.C:
					call, cancel := context.WithTimeout(ctx, 2*time.Second)
					e := child.CheckHealth(call)
					cancel()
					if e != nil {
						failures++
					} else {
						failures = 0
					}
					if failures >= 3 {
						running = false
					}
				}
			}
			exit.Stop()
			health.Stop()
			withdraw(child)
			emit("stopping", began)
			child.Close()
			if ctx.Err() != nil {
				emit("disabled", began)
				return
			}
			if time.Since(ready) >= 5*time.Minute {
				attempts = nil
				restarts = 0
			}
		}
		if ctx.Err() != nil {
			emit("disabled", began)
			return
		}
		now := time.Now()
		recent := attempts[:0]
		for _, t := range attempts {
			if now.Sub(t) < 5*time.Minute {
				recent = append(recent, t)
			}
		}
		attempts = recent
		if len(attempts) >= 5 {
			emit("failed", began)
			return
		}
		delay := time.Second << len(attempts)
		attempts = append(attempts, now)
		restarts++
		emit("backoff", began)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			emit("disabled", began)
			return
		case <-timer.C:
		}
	}
}

// StateLogger emits bounded fields. It never accepts arguments, URLs or credentials.
func StateLogger(plugin, capability string) func(Event) {
	return func(e Event) {
		slog.Info("plugin capability state", "plugin", plugin, "capability", capability, "state", e.State, "restarts", e.Restarts, "duration", e.Duration)
	}
}
