package state

import (
	"fmt"
	"testing"

	imap "github.com/meszmate/imap-go"
)

func TestNew(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)
	if m.State() != imap.ConnStateNotAuthenticated {
		t.Errorf("expected initial state NotAuthenticated, got %s", m.State())
	}
}

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    imap.ConnState
		to      imap.ConnState
		wantErr bool
	}{
		{"not auth -> auth", imap.ConnStateNotAuthenticated, imap.ConnStateAuthenticated, false},
		{"not auth -> logout", imap.ConnStateNotAuthenticated, imap.ConnStateLogout, false},
		{"not auth -> selected (invalid)", imap.ConnStateNotAuthenticated, imap.ConnStateSelected, true},
		{"auth -> selected", imap.ConnStateAuthenticated, imap.ConnStateSelected, false},
		{"auth -> logout", imap.ConnStateAuthenticated, imap.ConnStateLogout, false},
		{"auth -> not auth (unauth)", imap.ConnStateAuthenticated, imap.ConnStateNotAuthenticated, false},
		{"selected -> auth", imap.ConnStateSelected, imap.ConnStateAuthenticated, false},
		{"selected -> selected (reselect)", imap.ConnStateSelected, imap.ConnStateSelected, false},
		{"selected -> logout", imap.ConnStateSelected, imap.ConnStateLogout, false},
		{"selected -> not auth (invalid)", imap.ConnStateSelected, imap.ConnStateNotAuthenticated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.from)
			err := m.Transition(tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transition(%s -> %s) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
			if err == nil && m.State() != tt.to {
				t.Errorf("expected state %s after transition, got %s", tt.to, m.State())
			}
		})
	}
}

func TestRequireState(t *testing.T) {
	m := New(imap.ConnStateAuthenticated)

	if err := m.RequireState(imap.ConnStateAuthenticated); err != nil {
		t.Errorf("RequireState(Authenticated) should not fail: %v", err)
	}

	if err := m.RequireState(imap.ConnStateAuthenticated, imap.ConnStateSelected); err != nil {
		t.Errorf("RequireState(Authenticated, Selected) should not fail: %v", err)
	}

	if err := m.RequireState(imap.ConnStateSelected); err == nil {
		t.Error("RequireState(Selected) should fail when in Authenticated state")
	}
}

func TestBeforeHook(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)

	var hookCalled bool
	var hookFrom, hookTo imap.ConnState
	m.OnBefore(func(from, to imap.ConnState) error {
		hookCalled = true
		hookFrom = from
		hookTo = to
		return nil
	})

	if err := m.Transition(imap.ConnStateAuthenticated); err != nil {
		t.Fatal(err)
	}

	if !hookCalled {
		t.Error("before hook was not called")
	}
	if hookFrom != imap.ConnStateNotAuthenticated {
		t.Errorf("hook from = %s, want NotAuthenticated", hookFrom)
	}
	if hookTo != imap.ConnStateAuthenticated {
		t.Errorf("hook to = %s, want Authenticated", hookTo)
	}
}

func TestAfterHook(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)

	var hookCalled bool
	m.OnAfter(func(from, to imap.ConnState) error {
		hookCalled = true
		return nil
	})

	if err := m.Transition(imap.ConnStateAuthenticated); err != nil {
		t.Fatal(err)
	}

	if !hookCalled {
		t.Error("after hook was not called")
	}
}

func TestBeforeHookError(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)

	m.OnBefore(func(from, to imap.ConnState) error {
		return fmt.Errorf("hook error")
	})

	err := m.Transition(imap.ConnStateAuthenticated)
	if err == nil {
		t.Error("expected error from before hook")
	}

	// State should NOT have changed
	if m.State() != imap.ConnStateNotAuthenticated {
		t.Errorf("state should remain NotAuthenticated after before hook error, got %s", m.State())
	}
}

func TestCanTransition(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)

	if !m.CanTransition(imap.ConnStateAuthenticated) {
		t.Error("should be able to transition to Authenticated")
	}

	if m.CanTransition(imap.ConnStateSelected) {
		t.Error("should not be able to transition to Selected from NotAuthenticated")
	}
}

func TestAddTransition(t *testing.T) {
	m := New(imap.ConnStateLogout)

	// By default, no transitions from Logout
	if m.CanTransition(imap.ConnStateNotAuthenticated) {
		t.Error("should not be able to transition from Logout by default")
	}

	m.AddTransition(imap.ConnStateLogout, imap.ConnStateNotAuthenticated)

	if !m.CanTransition(imap.ConnStateNotAuthenticated) {
		t.Error("should be able to transition after AddTransition")
	}
}

func TestSetTransitions(t *testing.T) {
	m := New(imap.ConnStateNotAuthenticated)

	m.SetTransitions(map[imap.ConnState][]imap.ConnState{
		imap.ConnStateNotAuthenticated: {imap.ConnStateLogout},
	})

	if m.CanTransition(imap.ConnStateAuthenticated) {
		t.Error("should not be able to transition to Authenticated after SetTransitions")
	}

	if !m.CanTransition(imap.ConnStateLogout) {
		t.Error("should be able to transition to Logout")
	}
}

func TestCommandAllowedStates(t *testing.T) {
	tests := []struct {
		cmd     string
		wantLen int
	}{
		{"CAPABILITY", 3},
		{"NOOP", 3},
		{"LOGOUT", 3},
		{"LOGIN", 1},
		{"STARTTLS", 1},
		{"SELECT", 2},
		{"FETCH", 1},
		{"STORE", 1},
		{"UNKNOWN", 0},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			states := CommandAllowedStates(tt.cmd)
			if len(states) != tt.wantLen {
				t.Errorf("CommandAllowedStates(%s) returned %d states, want %d", tt.cmd, len(states), tt.wantLen)
			}
		})
	}
}
