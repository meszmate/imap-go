// Package state provides an explicit state machine for IMAP connection states.
//
// The state machine validates state transitions according to RFC 9051 and
// provides hooks for custom behavior during transitions.
package state

import (
	"fmt"
	"sync"

	imap "github.com/meszmate/imap-go"
)

// TransitionHook is a function called during state transitions.
type TransitionHook func(from, to imap.ConnState) error

// Machine manages IMAP connection state transitions.
type Machine struct {
	mu          sync.RWMutex
	state       imap.ConnState
	transitions map[imap.ConnState][]imap.ConnState
	beforeHooks []TransitionHook
	afterHooks  []TransitionHook
}

// New creates a new state machine starting in the given state.
func New(initial imap.ConnState) *Machine {
	return &Machine{
		state:       initial,
		transitions: DefaultTransitions(),
	}
}

// State returns the current state.
func (m *Machine) State() imap.ConnState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Transition attempts to transition to the target state.
// Returns an error if the transition is not allowed.
func (m *Machine) Transition(target imap.ConnState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.canTransition(m.state, target) {
		return fmt.Errorf("imap: invalid state transition from %s to %s", m.state, target)
	}

	from := m.state

	// Run before hooks
	for _, hook := range m.beforeHooks {
		if err := hook(from, target); err != nil {
			return fmt.Errorf("imap: before hook failed: %w", err)
		}
	}

	m.state = target

	// Run after hooks
	for _, hook := range m.afterHooks {
		if err := hook(from, target); err != nil {
			// State has already changed; after hooks failing is noted but
			// the transition stands
			return fmt.Errorf("imap: after hook failed: %w", err)
		}
	}

	return nil
}

// RequireState checks that the current state is one of the allowed states.
func (m *Machine) RequireState(allowed ...imap.ConnState) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range allowed {
		if m.state == s {
			return nil
		}
	}

	return fmt.Errorf("imap: command not allowed in %s state", m.state)
}

// OnBefore registers a hook that runs before each state transition.
func (m *Machine) OnBefore(hook TransitionHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beforeHooks = append(m.beforeHooks, hook)
}

// OnAfter registers a hook that runs after each state transition.
func (m *Machine) OnAfter(hook TransitionHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.afterHooks = append(m.afterHooks, hook)
}

// SetTransitions replaces the transition rules.
func (m *Machine) SetTransitions(transitions map[imap.ConnState][]imap.ConnState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitions = transitions
}

// AddTransition adds an allowed transition.
func (m *Machine) AddTransition(from, to imap.ConnState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitions[from] = append(m.transitions[from], to)
}

// CanTransition returns whether a transition from the current state to target is allowed.
func (m *Machine) CanTransition(target imap.ConnState) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.canTransition(m.state, target)
}

func (m *Machine) canTransition(from, to imap.ConnState) bool {
	allowed, ok := m.transitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
