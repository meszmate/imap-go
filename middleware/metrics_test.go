package middleware_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
)

// --- NewMetrics ---

func TestNewMetrics(t *testing.T) {
	m := middleware.NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	if m.CommandsTotal.Load() != 0 {
		t.Fatalf("expected CommandsTotal=0, got %d", m.CommandsTotal.Load())
	}
	if m.CommandErrors.Load() != 0 {
		t.Fatalf("expected CommandErrors=0, got %d", m.CommandErrors.Load())
	}
	if m.ActiveCommands.Load() != 0 {
		t.Fatalf("expected ActiveCommands=0, got %d", m.ActiveCommands.Load())
	}
}

// --- CommandCount for unknown command ---

func TestMetrics_CommandCount_Unknown(t *testing.T) {
	m := middleware.NewMetrics()
	count := m.CommandCount("NONEXISTENT")
	if count != 0 {
		t.Fatalf("expected 0 for unknown command, got %d", count)
	}
}

// --- CommandDuration for unknown command ---

func TestMetrics_CommandDuration_Unknown(t *testing.T) {
	m := middleware.NewMetrics()
	dur := m.CommandDuration("NONEXISTENT")
	if dur != 0 {
		t.Fatalf("expected 0 for unknown command duration, got %v", dur)
	}
}

// --- MetricsMiddleware: single command ---

func TestMetricsMiddleware_SingleCommand(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
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

	if m.CommandsTotal.Load() != 1 {
		t.Fatalf("expected CommandsTotal=1, got %d", m.CommandsTotal.Load())
	}
	if m.CommandErrors.Load() != 0 {
		t.Fatalf("expected CommandErrors=0, got %d", m.CommandErrors.Load())
	}
	if m.ActiveCommands.Load() != 0 {
		t.Fatalf("expected ActiveCommands=0 after completion, got %d", m.ActiveCommands.Load())
	}
	if m.CommandCount("NOOP") != 1 {
		t.Fatalf("expected CommandCount(NOOP)=1, got %d", m.CommandCount("NOOP"))
	}
}

// --- MetricsMiddleware: command error ---

func TestMetricsMiddleware_CommandError(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return errors.New("failed")
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "FETCH",
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if m.CommandsTotal.Load() != 1 {
		t.Fatalf("expected CommandsTotal=1, got %d", m.CommandsTotal.Load())
	}
	if m.CommandErrors.Load() != 1 {
		t.Fatalf("expected CommandErrors=1, got %d", m.CommandErrors.Load())
	}
	if m.ActiveCommands.Load() != 0 {
		t.Fatalf("expected ActiveCommands=0, got %d", m.ActiveCommands.Load())
	}
}

// --- MetricsMiddleware: multiple commands ---

func TestMetricsMiddleware_MultipleCommands(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	commands := []string{"NOOP", "FETCH", "NOOP", "STORE", "FETCH"}
	for _, name := range commands {
		ctx := &server.CommandContext{
			Context: context.Background(),
			Name:    name,
		}
		if err := handler.Handle(ctx); err != nil {
			t.Fatalf("unexpected error for %s: %v", name, err)
		}
	}

	if m.CommandsTotal.Load() != 5 {
		t.Fatalf("expected CommandsTotal=5, got %d", m.CommandsTotal.Load())
	}
	if m.CommandCount("NOOP") != 2 {
		t.Fatalf("expected CommandCount(NOOP)=2, got %d", m.CommandCount("NOOP"))
	}
	if m.CommandCount("FETCH") != 2 {
		t.Fatalf("expected CommandCount(FETCH)=2, got %d", m.CommandCount("FETCH"))
	}
	if m.CommandCount("STORE") != 1 {
		t.Fatalf("expected CommandCount(STORE)=1, got %d", m.CommandCount("STORE"))
	}
}

// --- MetricsMiddleware: mixed success and error ---

func TestMetricsMiddleware_MixedSuccessAndError(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	callCount := 0
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		callCount++
		if callCount%2 == 0 {
			return errors.New("even call error")
		}
		return nil
	}))

	for i := 0; i < 6; i++ {
		ctx := &server.CommandContext{
			Context: context.Background(),
			Name:    "CMD",
		}
		_ = handler.Handle(ctx)
	}

	if m.CommandsTotal.Load() != 6 {
		t.Fatalf("expected CommandsTotal=6, got %d", m.CommandsTotal.Load())
	}
	if m.CommandErrors.Load() != 3 {
		t.Fatalf("expected CommandErrors=3, got %d", m.CommandErrors.Load())
	}
}

// --- MetricsMiddleware: ActiveCommands during execution ---

func TestMetricsMiddleware_ActiveCommandsDuringExecution(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	var activeSnapshot int64
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		activeSnapshot = m.ActiveCommands.Load()
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "TEST",
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// During execution, ActiveCommands should have been 1
	if activeSnapshot != 1 {
		t.Fatalf("expected ActiveCommands=1 during execution, got %d", activeSnapshot)
	}

	// After execution, it should be 0
	if m.ActiveCommands.Load() != 0 {
		t.Fatalf("expected ActiveCommands=0 after execution, got %d", m.ActiveCommands.Load())
	}
}

// --- MetricsMiddleware: duration tracking ---

func TestMetricsMiddleware_DurationTracking(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "SLOW",
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dur := m.CommandDuration("SLOW")
	if dur < 5*time.Millisecond {
		t.Fatalf("expected duration >= 5ms, got %v", dur)
	}
}

// --- MetricsMiddleware: duration accumulates ---

func TestMetricsMiddleware_DurationAccumulates(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}))

	for i := 0; i < 3; i++ {
		ctx := &server.CommandContext{
			Context: context.Background(),
			Name:    "ACCUM",
		}
		if err := handler.Handle(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	dur := m.CommandDuration("ACCUM")
	if dur < 10*time.Millisecond {
		t.Fatalf("expected accumulated duration >= 10ms, got %v", dur)
	}
}

// --- MetricsMiddleware: error propagation ---

func TestMetricsMiddleware_ErrorPropagation(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	expectedErr := errors.New("specific error")
	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return expectedErr
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "FAIL",
	}

	err := handler.Handle(ctx)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

// --- MetricsMiddleware: per-command counts ---

func TestMetricsMiddleware_PerCommandCounts(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	commandCounts := map[string]int{
		"NOOP":   3,
		"FETCH":  5,
		"STORE":  2,
		"SELECT": 1,
	}

	for name, count := range commandCounts {
		for i := 0; i < count; i++ {
			ctx := &server.CommandContext{
				Context: context.Background(),
				Name:    name,
			}
			_ = handler.Handle(ctx)
		}
	}

	for name, expected := range commandCounts {
		got := m.CommandCount(name)
		if got != int64(expected) {
			t.Errorf("CommandCount(%s): expected %d, got %d", name, expected, got)
		}
	}

	totalExpected := int64(0)
	for _, c := range commandCounts {
		totalExpected += int64(c)
	}
	if m.CommandsTotal.Load() != totalExpected {
		t.Fatalf("expected CommandsTotal=%d, got %d", totalExpected, m.CommandsTotal.Load())
	}
}

// --- MetricsMiddleware: shared Metrics across multiple wrapped handlers ---

func TestMetricsMiddleware_SharedMetrics(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler1 := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))
	handler2 := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	ctx1 := &server.CommandContext{Context: context.Background(), Name: "CMD1"}
	ctx2 := &server.CommandContext{Context: context.Background(), Name: "CMD2"}

	_ = handler1.Handle(ctx1)
	_ = handler2.Handle(ctx2)
	_ = handler1.Handle(ctx1)

	if m.CommandsTotal.Load() != 3 {
		t.Fatalf("expected CommandsTotal=3, got %d", m.CommandsTotal.Load())
	}
	if m.CommandCount("CMD1") != 2 {
		t.Fatalf("expected CMD1 count=2, got %d", m.CommandCount("CMD1"))
	}
	if m.CommandCount("CMD2") != 1 {
		t.Fatalf("expected CMD2 count=1, got %d", m.CommandCount("CMD2"))
	}
}

// --- MetricsMiddleware: zero-duration handler ---

func TestMetricsMiddleware_ZeroDurationHandler(t *testing.T) {
	m := middleware.NewMetrics()
	mw := middleware.MetricsMiddleware(m)

	handler := mw(server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return nil
	}))

	ctx := &server.CommandContext{
		Context: context.Background(),
		Name:    "INSTANT",
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Duration should be >= 0 (non-negative)
	dur := m.CommandDuration("INSTANT")
	if dur < 0 {
		t.Fatalf("expected non-negative duration, got %v", dur)
	}
}
