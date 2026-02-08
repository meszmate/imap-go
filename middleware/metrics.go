package middleware

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/meszmate/imap-go/server"
)

// Metrics tracks server metrics.
type Metrics struct {
	// CommandsTotal is the total number of commands processed.
	CommandsTotal atomic.Int64
	// CommandErrors is the total number of command errors.
	CommandErrors atomic.Int64
	// ActiveCommands is the number of currently executing commands.
	ActiveCommands atomic.Int64

	mu              sync.RWMutex
	commandCounts   map[string]*atomic.Int64
	commandDuration map[string]*atomic.Int64 // nanoseconds total
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		commandCounts:   make(map[string]*atomic.Int64),
		commandDuration: make(map[string]*atomic.Int64),
	}
}

// CommandCount returns the total count for a specific command.
func (m *Metrics) CommandCount(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.commandCounts[name]; ok {
		return c.Load()
	}
	return 0
}

// CommandDuration returns the total duration for a specific command in nanoseconds.
func (m *Metrics) CommandDuration(name string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.commandDuration[name]; ok {
		return time.Duration(d.Load())
	}
	return 0
}

func (m *Metrics) getCounter(name string) *atomic.Int64 {
	m.mu.RLock()
	c, ok := m.commandCounts[name]
	m.mu.RUnlock()
	if ok {
		return c
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok = m.commandCounts[name]
	if ok {
		return c
	}
	c = &atomic.Int64{}
	m.commandCounts[name] = c
	return c
}

func (m *Metrics) getDuration(name string) *atomic.Int64 {
	m.mu.RLock()
	d, ok := m.commandDuration[name]
	m.mu.RUnlock()
	if ok {
		return d
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok = m.commandDuration[name]
	if ok {
		return d
	}
	d = &atomic.Int64{}
	m.commandDuration[name] = d
	return d
}

// MetricsMiddleware returns a middleware that records command metrics.
func MetricsMiddleware(metrics *Metrics) Middleware {
	return func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			metrics.CommandsTotal.Add(1)
			metrics.ActiveCommands.Add(1)
			metrics.getCounter(ctx.Name).Add(1)

			start := time.Now()
			err := next.Handle(ctx)
			duration := time.Since(start)

			metrics.ActiveCommands.Add(-1)
			metrics.getDuration(ctx.Name).Add(int64(duration))

			if err != nil {
				metrics.CommandErrors.Add(1)
			}

			return err
		})
	}
}
