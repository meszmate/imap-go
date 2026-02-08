package middleware_test

import (
	"strings"
	"testing"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
)

// --- Recovery: no panic ---

func TestRecovery_NoPanic(t *testing.T) {
	mw := middleware.Recovery()

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

// --- Recovery: handler returns error (not panic) ---

func TestRecovery_HandlerError(t *testing.T) {
	mw := middleware.Recovery()

	expectedErr := &testError{msg: "normal error"}
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

// --- Recovery: recovers from string panic ---

func TestRecovery_RecoverFromStringPanic(t *testing.T) {
	mw := middleware.Recovery()

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		panic("something went wrong")
	}))

	ctx, cleanup := newTestContext("TEST")
	defer cleanup()

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error after panic recovery, got nil")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Fatalf("expected 'internal server error' in error, got: %v", err)
	}
}

// --- Recovery: recovers from error panic ---

func TestRecovery_RecoverFromErrorPanic(t *testing.T) {
	mw := middleware.Recovery()

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		panic(&testError{msg: "panic error"})
	}))

	ctx, cleanup := newTestContext("TEST")
	defer cleanup()

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error after panic recovery, got nil")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Fatalf("expected 'internal server error', got: %v", err)
	}
}

// --- Recovery: recovers from integer panic ---

func TestRecovery_RecoverFromIntPanic(t *testing.T) {
	mw := middleware.Recovery()

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		panic(42)
	}))

	ctx, cleanup := newTestContext("TEST")
	defer cleanup()

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error after panic recovery, got nil")
	}
}

// --- Recovery: recovers from nil panic ---

func TestRecovery_RecoverFromNilPanic(t *testing.T) {
	mw := middleware.Recovery()

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		panic(nil)
	}))

	ctx, cleanup := newTestContext("TEST")
	defer cleanup()

	// With panic(nil), recover() returns nil, so the defer block
	// won't set an error. The behavior depends on Go version.
	// In Go 1.21+, panic(nil) is wrapped as *runtime.PanicNilError.
	// We just verify it does not crash the test process.
	_ = handler.Handle(ctx)
}

// --- Recovery: returns IMAPError (NO type) ---

func TestRecovery_ReturnsIMAPError(t *testing.T) {
	mw := middleware.Recovery()

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		panic("boom")
	}))

	ctx, cleanup := newTestContext("FETCH")
	defer cleanup()

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should contain "NO" response type (from imap.ErrNo)
	errStr := err.Error()
	if !strings.Contains(errStr, "NO") {
		t.Fatalf("expected NO response error, got: %v", errStr)
	}
}

// --- Recovery: does not affect subsequent calls ---

func TestRecovery_SubsequentCallsWork(t *testing.T) {
	mw := middleware.Recovery()

	callCount := 0
	panicOnFirst := true

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		callCount++
		if panicOnFirst {
			panicOnFirst = false
			panic("first call panic")
		}
		return nil
	}))

	ctx, cleanup := newTestContext("CMD")
	defer cleanup()

	// First call panics
	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Second call should work normally
	err = handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}
