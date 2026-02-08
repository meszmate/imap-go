// Package auth provides pluggable SASL authentication mechanisms for IMAP.
package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ClientMechanism is a client-side SASL authentication mechanism.
type ClientMechanism interface {
	// Name returns the SASL mechanism name (e.g., "PLAIN", "SCRAM-SHA-256").
	Name() string
	// Start begins authentication and returns the initial response.
	// If no initial response is needed, ir is nil.
	Start() (ir []byte, err error)
	// Next processes a server challenge and returns the client response.
	Next(challenge []byte) (response []byte, err error)
}

// ServerMechanism is a server-side SASL authentication mechanism.
type ServerMechanism interface {
	// Name returns the SASL mechanism name.
	Name() string
	// Next processes a client response and returns the next challenge.
	// If done is true, authentication is complete (successfully or with an error).
	Next(response []byte) (challenge []byte, done bool, err error)
}

// Authenticator validates credentials from SASL authentication.
type Authenticator interface {
	// Authenticate validates the given identity and credentials.
	Authenticate(ctx context.Context, mechanism, identity string, credentials []byte) error
}

// AuthenticatorFunc is an adapter for Authenticator.
type AuthenticatorFunc func(ctx context.Context, mechanism, identity string, credentials []byte) error

// Authenticate implements Authenticator.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, mechanism, identity string, credentials []byte) error {
	return f(ctx, mechanism, identity, credentials)
}

// Registry manages available authentication mechanisms.
type Registry struct {
	mu              sync.RWMutex
	clientFactories map[string]ClientMechanismFactory
	serverFactories map[string]ServerMechanismFactory
}

// ClientMechanismFactory creates a new client mechanism instance.
type ClientMechanismFactory func() ClientMechanism

// ServerMechanismFactory creates a new server mechanism instance with an authenticator.
type ServerMechanismFactory func(auth Authenticator) ServerMechanism

// NewRegistry creates a new auth mechanism registry.
func NewRegistry() *Registry {
	return &Registry{
		clientFactories: make(map[string]ClientMechanismFactory),
		serverFactories: make(map[string]ServerMechanismFactory),
	}
}

// RegisterClient registers a client mechanism factory.
func (r *Registry) RegisterClient(name string, factory ClientMechanismFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientFactories[strings.ToUpper(name)] = factory
}

// RegisterServer registers a server mechanism factory.
func (r *Registry) RegisterServer(name string, factory ServerMechanismFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serverFactories[strings.ToUpper(name)] = factory
}

// NewClientMechanism creates a new client mechanism by name.
func (r *Registry) NewClientMechanism(name string) (ClientMechanism, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.clientFactories[strings.ToUpper(name)]
	if !ok {
		return nil, fmt.Errorf("auth: unsupported client mechanism %q", name)
	}
	return factory(), nil
}

// NewServerMechanism creates a new server mechanism by name.
func (r *Registry) NewServerMechanism(name string, auth Authenticator) (ServerMechanism, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.serverFactories[strings.ToUpper(name)]
	if !ok {
		return nil, fmt.Errorf("auth: unsupported server mechanism %q", name)
	}
	return factory(auth), nil
}

// ClientMechanisms returns the names of all registered client mechanisms.
func (r *Registry) ClientMechanisms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.clientFactories))
	for name := range r.clientFactories {
		names = append(names, name)
	}
	return names
}

// ServerMechanisms returns the names of all registered server mechanisms.
func (r *Registry) ServerMechanisms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.serverFactories))
	for name := range r.serverFactories {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global default registry with built-in mechanisms.
var DefaultRegistry = NewRegistry()
