package middleware

import (
	"sync"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// MaxCommandsPerSecond is the maximum number of commands per second per connection.
	MaxCommandsPerSecond float64
	// BurstSize is the maximum burst size.
	BurstSize int
}

// RateLimit returns a middleware that rate limits commands per connection.
func RateLimit(config RateLimitConfig) Middleware {
	if config.MaxCommandsPerSecond <= 0 {
		config.MaxCommandsPerSecond = 100
	}
	if config.BurstSize <= 0 {
		config.BurstSize = 10
	}

	type limiterState struct {
		tokens    float64
		lastCheck time.Time
	}

	var mu sync.Mutex
	limiters := make(map[string]*limiterState)

	return func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			key := ctx.Conn.RemoteAddr().String()

			mu.Lock()
			state, ok := limiters[key]
			if !ok {
				state = &limiterState{
					tokens:    float64(config.BurstSize),
					lastCheck: time.Now(),
				}
				limiters[key] = state
			}

			now := time.Now()
			elapsed := now.Sub(state.lastCheck).Seconds()
			state.lastCheck = now
			state.tokens += elapsed * config.MaxCommandsPerSecond
			if state.tokens > float64(config.BurstSize) {
				state.tokens = float64(config.BurstSize)
			}

			if state.tokens < 1 {
				mu.Unlock()
				return imap.ErrBad("rate limit exceeded")
			}
			state.tokens--
			mu.Unlock()

			return next.Handle(ctx)
		})
	}
}
