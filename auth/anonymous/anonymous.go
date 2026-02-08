// Package anonymous implements the ANONYMOUS SASL mechanism (RFC 4505).
package anonymous

import (
	"context"
	"fmt"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "ANONYMOUS"

// ClientMechanism implements ANONYMOUS authentication for clients.
type ClientMechanism struct {
	// Trace is an optional trace token (e.g., email address).
	Trace string
}

// Name returns "ANONYMOUS".
func (m *ClientMechanism) Name() string { return Name }

// Start returns the trace token.
func (m *ClientMechanism) Start() ([]byte, error) {
	return []byte(m.Trace), nil
}

// Next is not called for ANONYMOUS.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	return nil, fmt.Errorf("anonymous: unexpected challenge")
}

// ServerMechanism implements ANONYMOUS authentication for servers.
type ServerMechanism struct {
	auth auth.Authenticator
	done bool
}

// NewServerMechanism creates a new server-side ANONYMOUS mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "ANONYMOUS".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the client response.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	if m.done {
		return nil, true, fmt.Errorf("anonymous: mechanism already completed")
	}
	m.done = true

	trace := string(response)
	err := m.auth.Authenticate(context.Background(), Name, trace, nil)
	return nil, true, err
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
