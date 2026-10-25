package runtime

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

type stoppedProcess struct{}

func (stoppedProcess) Close()                            {}
func (stoppedProcess) Exited() bool                      { return true }
func (stoppedProcess) CheckHealth(context.Context) error { return nil }
func TestSupervisorBudgetAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		states := []string{}
		start := time.Now()
		Supervise(t.Context(), func(context.Context) (stoppedProcess, error) {
			calls++
			return stoppedProcess{}, errors.New("safe failure")
		}, func(stoppedProcess) {}, func(stoppedProcess) {}, func(e Event) { states = append(states, e.State) })
		if calls != 6 || time.Since(start) != 31*time.Second || states[len(states)-1] != "failed" {
			t.Fatalf("calls %d elapsed %v states %v", calls, time.Since(start), states)
		}
		ctx, cancel := context.WithCancel(t.Context())
		calls = 0
		Supervise(ctx, func(context.Context) (stoppedProcess, error) {
			calls++
			cancel()
			return stoppedProcess{}, errors.New("failed")
		}, func(stoppedProcess) {}, func(stoppedProcess) {}, nil)
		if calls != 1 {
			t.Fatalf("restart after cancellation: %d", calls)
		}
	})
}

type unhealthyProcess struct{ Checks, Closed int }

func (p *unhealthyProcess) Close()       { p.Closed++ }
func (p *unhealthyProcess) Exited() bool { return false }
func (p *unhealthyProcess) CheckHealth(ctx context.Context) error {
	p.Checks++
	<-ctx.Done()
	return ctx.Err()
}
func TestSupervisorThreeHealthFailuresUseIndependentDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		child := &unhealthyProcess{}
		starts, withdrawals := 0, 0
		began := time.Now()
		Supervise(ctx, func(context.Context) (*unhealthyProcess, error) { starts++; return child, nil }, func(*unhealthyProcess) {}, func(*unhealthyProcess) { withdrawals++ }, func(e Event) {
			if e.State == "backoff" {
				cancel()
			}
		})
		if starts != 1 || child.Checks != 3 || child.Closed != 1 || withdrawals != 1 || time.Since(began) != 32*time.Second {
			t.Fatalf("starts %d checks %d closed %d withdrawals %d elapsed %v", starts, child.Checks, child.Closed, withdrawals, time.Since(began))
		}
	})
}

type healthyProcess struct{ Since time.Time }

func (p healthyProcess) Close()                            {}
func (p healthyProcess) Exited() bool                      { return time.Since(p.Since) >= 301*time.Second }
func (p healthyProcess) CheckHealth(context.Context) error { return nil }
func TestFiveHealthyMinutesResetRestartBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		starts := 0
		healthy := false
		resetRestarts := 0
		Supervise(ctx, func(context.Context) (healthyProcess, error) {
			starts++
			if starts < 4 {
				return healthyProcess{}, errors.New("startup")
			}
			return healthyProcess{time.Now()}, nil
		}, func(healthyProcess) { healthy = true }, func(healthyProcess) {}, func(e Event) {
			if healthy && e.State == "backoff" {
				resetRestarts = e.Restarts
				cancel()
			}
		})
		if starts != 4 || resetRestarts != 1 {
			t.Fatalf("healthy budget was not reset: starts %d restarts %d", starts, resetRestarts)
		}
	})
}
