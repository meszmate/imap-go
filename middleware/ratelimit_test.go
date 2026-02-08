package middleware_test

import (
	"strings"
	"testing"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
)

// --- RateLimit with default config ---

func TestRateLimit_DefaultConfig(t *testing.T) {
	mw := middleware.RateLimit(middleware.RateLimitConfig{})

	called := false
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		called = true
		return nil
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// --- RateLimit allows burst ---

func TestRateLimit_AllowsBurst(t *testing.T) {
	burstSize := 5
	mw := middleware.RateLimit(middleware.RateLimitConfig{
		MaxCommandsPerSecond: 1,
		BurstSize:            burstSize,
	})

	handlerCallCount := 0
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		handlerCallCount++
		return nil
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	// Should allow up to burstSize commands immediately
	for i := 0; i < burstSize; i++ {
		err := handler.Handle(ctx)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}

	if handlerCallCount != burstSize {
		t.Fatalf("expected %d calls, got %d", burstSize, handlerCallCount)
	}
}

// --- RateLimit rejects after burst exhausted ---

func TestRateLimit_RejectsAfterBurstExhausted(t *testing.T) {
	burstSize := 3
	mw := middleware.RateLimit(middleware.RateLimitConfig{
		MaxCommandsPerSecond: 0.001, // very slow replenish
		BurstSize:            burstSize,
	})

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	// Exhaust the burst
	for i := 0; i < burstSize; i++ {
		err := handler.Handle(ctx)
		if err != nil {
			t.Fatalf("burst request %d: unexpected error: %v", i, err)
		}
	}

	// Next request should be rate limited
	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected 'rate limit' error, got: %v", err)
	}
}

// --- RateLimit keyed by remote address ---

func TestRateLimit_KeyedByRemoteAddr(t *testing.T) {
	// Note: net.Pipe() returns connections with the same "pipe" address,
	// so this test verifies that rate limiting is applied per remote address
	// by using a single connection.
	burstSize := 3
	mw := middleware.RateLimit(middleware.RateLimitConfig{
		MaxCommandsPerSecond: 0.001,
		BurstSize:            burstSize,
	})

	handlerCallCount := 0
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		handlerCallCount++
		return nil
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	// Exhaust burst
	for i := 0; i < burstSize; i++ {
		if err := handler.Handle(ctx); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}

	// Should be rate limited now
	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}

	if handlerCallCount != burstSize {
		t.Fatalf("expected %d handler calls, got %d", burstSize, handlerCallCount)
	}
}

// --- RateLimit passes through errors from handler ---

func TestRateLimit_PassesThroughHandlerError(t *testing.T) {
	mw := middleware.RateLimit(middleware.RateLimitConfig{
		MaxCommandsPerSecond: 100,
		BurstSize:            10,
	})

	expectedErr := &testError{msg: "handler failed"}
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return expectedErr
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	err := handler.Handle(ctx)
	if err != expectedErr {
		t.Fatalf("expected handler error, got: %v", err)
	}
}

// --- RateLimit with negative/zero config values uses defaults ---

func TestRateLimit_NegativeConfigUsesDefaults(t *testing.T) {
	mw := middleware.RateLimit(middleware.RateLimitConfig{
		MaxCommandsPerSecond: -5,
		BurstSize:            -3,
	})

	called := false
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		called = true
		return nil
	}))

	ctx, cleanup := newTestContext("NOOP")
	defer cleanup()

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}
