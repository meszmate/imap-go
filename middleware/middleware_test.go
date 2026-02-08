package middleware_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
)

// newTestContext creates a CommandContext with a real Conn backed by net.Pipe.
// The returned cleanup function must be called to close the pipe connections.
func newTestContext(name string) (*server.CommandContext, func()) {
	clientConn, serverConn := net.Pipe()
	conn := server.NewTestConn(serverConn, slog.Default())

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    name,
		Conn:    conn,
	}

	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	}

	return ctx, cleanup
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// --- Chain ---

func TestChain_Empty(t *testing.T) {
	chain := middleware.Chain()

	called := false
	inner := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		called = true
		return nil
	})

	handler := chain(inner)
	if err := handler.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called")
	}
}

func TestChain_SingleMiddleware(t *testing.T) {
	var order []string

	mw := func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			order = append(order, "mw-before")
			err := next.Handle(ctx)
			order = append(order, "mw-after")
			return err
		})
	}

	chain := middleware.Chain(mw)
	inner := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		order = append(order, "inner")
		return nil
	})

	handler := chain(inner)
	if err := handler.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw-before", "inner", "mw-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("call %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestChain_MultipleMiddlewares_Order(t *testing.T) {
	var order []string

	makeMW := func(name string) middleware.Middleware {
		return func(next server.CommandHandler) server.CommandHandler {
			return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
				order = append(order, name+"-before")
				err := next.Handle(ctx)
				order = append(order, name+"-after")
				return err
			})
		}
	}

	// Chain(mw1, mw2, mw3): mw1 is outermost, mw3 is innermost
	chain := middleware.Chain(makeMW("mw1"), makeMW("mw2"), makeMW("mw3"))

	inner := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		order = append(order, "handler")
		return nil
	})

	handler := chain(inner)
	if err := handler.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"mw1-before", "mw2-before", "mw3-before",
		"handler",
		"mw3-after", "mw2-after", "mw1-after",
	}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("call %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestChain_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("handler error")

	mw := func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return next.Handle(ctx)
		})
	}

	chain := middleware.Chain(mw, mw)
	inner := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return expectedErr
	})

	handler := chain(inner)
	err := handler.Handle(nil)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestChain_MiddlewareCanShortCircuit(t *testing.T) {
	innerCalled := false
	shortCircuitErr := errors.New("short circuit")

	mw := func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return shortCircuitErr // do not call next
		})
	}

	chain := middleware.Chain(mw)
	inner := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		innerCalled = true
		return nil
	})

	handler := chain(inner)
	err := handler.Handle(nil)
	if err != shortCircuitErr {
		t.Fatalf("expected short circuit error, got %v", err)
	}
	if innerCalled {
		t.Fatal("inner handler should not have been called when middleware short-circuits")
	}
}

func TestChain_ReusableOnMultipleHandlers(t *testing.T) {
	callCount := 0

	mw := func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			callCount++
			return next.Handle(ctx)
		})
	}

	chain := middleware.Chain(mw)

	handler1 := chain(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))
	handler2 := chain(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	_ = handler1.Handle(nil)
	_ = handler2.Handle(nil)

	if callCount != 2 {
		t.Fatalf("expected mw called 2 times, got %d", callCount)
	}
}

// --- Chain applied multiple times ---

func TestChain_NestedChains(t *testing.T) {
	var order []string

	makeMW := func(name string) middleware.Middleware {
		return func(next server.CommandHandler) server.CommandHandler {
			return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
				order = append(order, name)
				return next.Handle(ctx)
			})
		}
	}

	inner := middleware.Chain(makeMW("inner1"), makeMW("inner2"))
	outer := middleware.Chain(makeMW("outer1"), inner)

	handler := outer(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		order = append(order, "handler")
		return nil
	}))

	_ = handler.Handle(nil)

	expected := []string{"outer1", "inner1", "inner2", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("call %d: expected %q, got %q", i, v, order[i])
		}
	}
}
