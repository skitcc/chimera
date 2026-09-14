package health

import (
	"context"
	"sync"
	"time"

	"chimera/internal/infra/worker"
)

type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type Check struct {
	Name string
	Ping func(context.Context) error
}

type Monitor struct {
	log      Logger
	interval time.Duration
	timeout  time.Duration
	workers  *worker.Pool
	checks   []Check

	mu    sync.RWMutex
	ready map[string]bool
}

func NewMonitor(log Logger, interval, timeout time.Duration, workers *worker.Pool, checks []Check) *Monitor {
	ready := make(map[string]bool, len(checks))
	for _, c := range checks {
		ready[c.Name] = true
	}
	return &Monitor{
		log:      log,
		interval: interval,
		timeout:  timeout,
		workers:  workers,
		checks:   checks,
		ready:    ready,
	}
}

func Wait(ctx context.Context, log Logger, name string, interval, timeout time.Duration, ping func(context.Context) error) error {
	for {
		err := pingOnce(ctx, timeout, ping)
		if err == nil {
			log.InfoContext(ctx, "dependency ready", "name", name)
			return nil
		}
		log.ErrorContext(ctx, "dependency unavailable", "name", name, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (m *Monitor) Run(ctx context.Context) {
	m.probe(ctx)
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.probe(ctx)
		}
	}
}

func (m *Monitor) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ok := range m.ready {
		if !ok {
			return false
		}
	}
	return len(m.ready) > 0
}

func (m *Monitor) Checks() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool, len(m.ready))
	for k, v := range m.ready {
		out[k] = v
	}
	return out
}

func (m *Monitor) probe(ctx context.Context) {
	var wg sync.WaitGroup
	for _, check := range m.checks {
		wg.Add(1)
		if !m.workers.Submit(ctx, func(ctx context.Context) {
			defer wg.Done()
			m.runCheck(ctx, check)
		}) {
			wg.Done()
		}
	}
	wg.Wait()
}

func (m *Monitor) runCheck(ctx context.Context, check Check) {
	err := pingOnce(ctx, m.timeout, check.Ping)
	m.mu.Lock()
	wasReady := m.ready[check.Name]
	m.ready[check.Name] = err == nil
	m.mu.Unlock()

	if err != nil {
		m.log.ErrorContext(ctx, "dependency down", "name", check.Name, "error", err)
		return
	}
	if !wasReady {
		m.log.InfoContext(ctx, "dependency recovered", "name", check.Name)
	}
}

func pingOnce(ctx context.Context, timeout time.Duration, ping func(context.Context) error) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return ping(cctx)
}
