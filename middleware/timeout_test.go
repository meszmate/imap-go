package middleware_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
)

// --- Timeout: handler completes before timeout ---

func TestTimeout_CompletesBeforeTimeout(t *testing.T) {
	mw := middleware.Timeout(1 * time.Second)

	called := false
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		called = true
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "NOOP",
	}

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// --- Timeout: handler returns error before timeout ---

func TestTimeout_HandlerError(t *testing.T) {
	mw := middleware.Timeout(1 * time.Second)

	expectedErr := &testError{msg: "handler failed"}
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return expectedErr
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "TEST",
	}

	err := handler.Handle(ctx)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

// --- Timeout: handler exceeds timeout ---

func TestTimeout_ExceedsTimeout(t *testing.T) {
	mw := middleware.Timeout(50 * time.Millisecond)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		// Simulate a long-running operation
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Context.Done():
			return ctx.Context.Err()
		}
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "SLOW",
	}

	start := time.Now()
	err := handler.Handle(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected 'timed out' error, got: %v", err)
	}

	// Verify it timed out roughly at the expected time
	if elapsed > 500*time.Millisecond {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

// --- Timeout: context is set with deadline ---

func TestTimeout_ContextHasDeadline(t *testing.T) {
	timeoutDuration := 100 * time.Millisecond
	mw := middleware.Timeout(timeoutDuration)

	var contextHadDeadline bool
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		_, contextHadDeadline = ctx.Context.Deadline()
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "CHECK",
	}

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contextHadDeadline {
		t.Fatal("expected context to have a deadline")
	}
}

// --- Timeout: fast handler with very short timeout ---

func TestTimeout_FastHandlerWithShortTimeout(t *testing.T) {
	mw := middleware.Timeout(10 * time.Millisecond)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		// Handler completes instantly
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "FAST",
	}

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Timeout: multiple sequential calls ---

func TestTimeout_MultipleSequentialCalls(t *testing.T) {
	mw := middleware.Timeout(100 * time.Millisecond)

	callCount := 0
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		callCount++
		return nil
	}))

	for i := 0; i < 5; i++ {
		ctx := &server.CommandContext{
			Context: context.Background(),
			Name:    "REPEAT",
		}

		err := handler.Handle(ctx)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	if callCount != 5 {
		t.Fatalf("expected 5 calls, got %d", callCount)
	}
}

// --- Timeout: handler blocked ignoring context ---

func TestTimeout_HandlerBlockedIgnoringContext(t *testing.T) {
	mw := middleware.Timeout(50 * time.Millisecond)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		// Simulate a handler that blocks but does NOT check context cancellation.
		// The timeout middleware should still return the timeout error
		// because it uses select on the done channel vs timeout context.
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "BLOCKED",
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected 'timed out' error, got: %v", err)
	}
}

// --- Timeout: returns NO IMAP error ---

func TestTimeout_ReturnsIMAPNoError(t *testing.T) {
	mw := middleware.Timeout(10 * time.Millisecond)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "SLOW",
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "NO") {
		t.Fatalf("expected NO error type, got: %s", errStr)
	}
}

// --- Timeout: pre-cancelled context ---

func TestTimeout_PreCancelledContext(t *testing.T) {
	mw := middleware.Timeout(1 * time.Second)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		<-ctx.Context.Done()
		return ctx.Context.Err()
	}))

	parentCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ctx := &server.CommandContext{
		Context: parentCtx,
		Name:    "CANCELLED",
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error with pre-cancelled context, got nil")
	}
}
