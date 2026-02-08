package server

import (
	"errors"
	"sort"
	"testing"
)

// --- Dispatcher tests ---

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
	if d.handlers == nil {
		t.Fatal("handlers map is nil")
	}
	if len(d.handlers) != 0 {
		t.Fatalf("expected 0 handlers, got %d", len(d.handlers))
	}
}

func TestDispatcherRegister(t *testing.T) {
	d := NewDispatcher()

	called := false
	handler := CommandHandlerFunc(func(ctx *CommandContext) error {
		called = true
		return nil
	})

	d.Register("LOGIN", handler)

	got := d.Get("LOGIN")
	if got == nil {
		t.Fatal("expected handler, got nil")
	}
	if err := got.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestDispatcherRegister_CaseInsensitive(t *testing.T) {
	d := NewDispatcher()
	handler := CommandHandlerFunc(func(ctx *CommandContext) error {
		return nil
	})

	d.Register("login", handler)

	// Should be stored as uppercase
	if got := d.Get("LOGIN"); got == nil {
		t.Fatal("Get(LOGIN) returned nil after Register(login)")
	}
	if got := d.Get("login"); got == nil {
		t.Fatal("Get(login) returned nil after Register(login)")
	}
	if got := d.Get("Login"); got == nil {
		t.Fatal("Get(Login) returned nil after Register(login)")
	}
}

func TestDispatcherRegister_OverwriteExisting(t *testing.T) {
	d := NewDispatcher()

	firstCalled := false
	first := CommandHandlerFunc(func(ctx *CommandContext) error {
		firstCalled = true
		return nil
	})

	secondCalled := false
	second := CommandHandlerFunc(func(ctx *CommandContext) error {
		secondCalled = true
		return nil
	})

	d.Register("TEST", first)
	d.Register("TEST", second)

	got := d.Get("TEST")
	if got == nil {
		t.Fatal("expected handler, got nil")
	}
	_ = got.Handle(nil)
	if firstCalled {
		t.Fatal("first handler should not have been called after overwrite")
	}
	if !secondCalled {
		t.Fatal("second handler should have been called")
	}
}

func TestDispatcherRegisterFunc(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.RegisterFunc("NOOP", func(ctx *CommandContext) error {
		called = true
		return nil
	})

	got := d.Get("NOOP")
	if got == nil {
		t.Fatal("expected handler, got nil")
	}
	if err := got.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler function was not called")
	}
}

func TestDispatcherGet_NotRegistered(t *testing.T) {
	d := NewDispatcher()

	got := d.Get("NONEXISTENT")
	if got != nil {
		t.Fatalf("expected nil for unregistered command, got %v", got)
	}
}

func TestDispatcherGet_CaseInsensitive(t *testing.T) {
	d := NewDispatcher()

	d.RegisterFunc("SELECT", func(ctx *CommandContext) error {
		return nil
	})

	tests := []string{"SELECT", "select", "Select", "sElEcT"}
	for _, name := range tests {
		if got := d.Get(name); got == nil {
			t.Errorf("Get(%q) returned nil, want non-nil", name)
		}
	}
}

func TestDispatcherWrap(t *testing.T) {
	d := NewDispatcher()

	var order []string

	inner := CommandHandlerFunc(func(ctx *CommandContext) error {
		order = append(order, "inner")
		return nil
	})

	d.Register("FETCH", inner)

	d.Wrap("FETCH", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			order = append(order, "before")
			err := next.Handle(ctx)
			order = append(order, "after")
			return err
		})
	})

	got := d.Get("FETCH")
	if got == nil {
		t.Fatal("expected wrapped handler, got nil")
	}

	if err := got.Handle(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(order), order)
	}
	if order[0] != "before" || order[1] != "inner" || order[2] != "after" {
		t.Fatalf("unexpected call order: %v", order)
	}
}

func TestDispatcherWrap_CaseInsensitive(t *testing.T) {
	d := NewDispatcher()

	called := false
	d.RegisterFunc("TEST", func(ctx *CommandContext) error {
		return nil
	})

	d.Wrap("test", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			called = true
			return next.Handle(ctx)
		})
	})

	got := d.Get("TEST")
	if got == nil {
		t.Fatal("expected handler after wrap")
	}
	_ = got.Handle(nil)
	if !called {
		t.Fatal("wrapper was not called")
	}
}

func TestDispatcherWrap_Noop_WhenNotRegistered(t *testing.T) {
	d := NewDispatcher()

	// Wrap on a non-existent handler should be a no-op
	d.Wrap("NONEXISTENT", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			return errors.New("should not be called")
		})
	})

	got := d.Get("NONEXISTENT")
	if got != nil {
		t.Fatalf("expected nil for non-existent command, got %v", got)
	}
}

func TestDispatcherWrap_MultipleWraps(t *testing.T) {
	d := NewDispatcher()

	var order []string

	d.RegisterFunc("CMD", func(ctx *CommandContext) error {
		order = append(order, "handler")
		return nil
	})

	d.Wrap("CMD", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			order = append(order, "wrap1-before")
			err := next.Handle(ctx)
			order = append(order, "wrap1-after")
			return err
		})
	})

	d.Wrap("CMD", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			order = append(order, "wrap2-before")
			err := next.Handle(ctx)
			order = append(order, "wrap2-after")
			return err
		})
	})

	got := d.Get("CMD")
	_ = got.Handle(nil)

	expected := []string{"wrap2-before", "wrap1-before", "handler", "wrap1-after", "wrap2-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("call %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestDispatcherWrap_ErrorPropagation(t *testing.T) {
	d := NewDispatcher()

	expectedErr := errors.New("test error")
	d.RegisterFunc("CMD", func(ctx *CommandContext) error {
		return expectedErr
	})

	d.Wrap("CMD", func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx *CommandContext) error {
			return next.Handle(ctx)
		})
	})

	got := d.Get("CMD")
	err := got.Handle(nil)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestDispatcherNames(t *testing.T) {
	d := NewDispatcher()

	d.RegisterFunc("FETCH", func(ctx *CommandContext) error { return nil })
	d.RegisterFunc("STORE", func(ctx *CommandContext) error { return nil })
	d.RegisterFunc("SEARCH", func(ctx *CommandContext) error { return nil })

	names := d.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}

	sort.Strings(names)
	expected := []string{"FETCH", "SEARCH", "STORE"}
	for i, v := range expected {
		if names[i] != v {
			t.Fatalf("name %d: expected %q, got %q", i, v, names[i])
		}
	}
}

func TestDispatcherNames_Empty(t *testing.T) {
	d := NewDispatcher()

	names := d.Names()
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d: %v", len(names), names)
	}
}

func TestDispatcherNames_UppercaseKeys(t *testing.T) {
	d := NewDispatcher()

	d.RegisterFunc("lowercase", func(ctx *CommandContext) error { return nil })
	d.RegisterFunc("MiXeD", func(ctx *CommandContext) error { return nil })

	names := d.Names()
	sort.Strings(names)

	expected := []string{"LOWERCASE", "MIXED"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
	for i, v := range expected {
		if names[i] != v {
			t.Fatalf("name %d: expected %q, got %q", i, v, names[i])
		}
	}
}

// --- parseLine tests ---

func TestParseLine(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTag   string
		wantName  string
		wantRest  string
		wantError bool
	}{
		{
			name:     "simple command",
			input:    "A001 LOGIN",
			wantTag:  "A001",
			wantName: "LOGIN",
			wantRest: "",
		},
		{
			name:     "command with arguments",
			input:    "A001 LOGIN user pass",
			wantTag:  "A001",
			wantName: "LOGIN",
			wantRest: "user pass",
		},
		{
			name:     "command with complex rest",
			input:    `A002 FETCH 1:* (FLAGS UID)`,
			wantTag:  "A002",
			wantName: "FETCH",
			wantRest: "1:* (FLAGS UID)",
		},
		{
			name:      "empty input",
			input:     "",
			wantError: true,
		},
		{
			name:      "tag only",
			input:     "A001",
			wantError: true,
		},
		{
			name:     "tag with trailing space and command",
			input:    "tag NOOP",
			wantTag:  "tag",
			wantName: "NOOP",
			wantRest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, name, rest, err := parseLine(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tag != tt.wantTag {
				t.Errorf("tag: got %q, want %q", tag, tt.wantTag)
			}
			if name != tt.wantName {
				t.Errorf("name: got %q, want %q", name, tt.wantName)
			}
			if rest != tt.wantRest {
				t.Errorf("rest: got %q, want %q", rest, tt.wantRest)
			}
		})
	}
}

// --- CommandHandlerFunc tests ---

func TestCommandHandlerFunc_Handle(t *testing.T) {
	called := false
	var fn CommandHandlerFunc = func(ctx *CommandContext) error {
		called = true
		return nil
	}

	err := fn.Handle(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler function was not called")
	}
}

func TestCommandHandlerFunc_HandleError(t *testing.T) {
	expectedErr := errors.New("handler error")
	var fn CommandHandlerFunc = func(ctx *CommandContext) error {
		return expectedErr
	}

	err := fn.Handle(nil)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

// --- CommandContext tests ---

func TestCommandContext_SetValueAndValue(t *testing.T) {
	ctx := &CommandContext{}

	ctx.SetValue("key1", "value1")
	ctx.SetValue("key2", 42)

	v1, ok := ctx.Value("key1")
	if !ok {
		t.Fatal("key1 not found")
	}
	if v1 != "value1" {
		t.Fatalf("key1: expected %q, got %v", "value1", v1)
	}

	v2, ok := ctx.Value("key2")
	if !ok {
		t.Fatal("key2 not found")
	}
	if v2 != 42 {
		t.Fatalf("key2: expected %d, got %v", 42, v2)
	}
}

func TestCommandContext_ValueNotFound(t *testing.T) {
	ctx := &CommandContext{}

	_, ok := ctx.Value("nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent key")
	}
}

func TestCommandContext_SetValue_Overwrite(t *testing.T) {
	ctx := &CommandContext{}

	ctx.SetValue("key", "original")
	ctx.SetValue("key", "updated")

	v, ok := ctx.Value("key")
	if !ok {
		t.Fatal("key not found")
	}
	if v != "updated" {
		t.Fatalf("expected %q, got %v", "updated", v)
	}
}
