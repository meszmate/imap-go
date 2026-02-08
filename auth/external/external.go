// Package external implements the EXTERNAL SASL mechanism (RFC 4422).
// This mechanism delegates authentication to an external channel (e.g., TLS client cert).
package external

import (
	"context"
	"fmt"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "EXTERNAL"

// ClientMechanism implements EXTERNAL authentication for clients.
type ClientMechanism struct {
	// AuthzID is the authorization identity (may be empty).
	AuthzID string
}

// Name returns "EXTERNAL".
func (m *ClientMechanism) Name() string { return Name }

// Start returns the authorization identity.
func (m *ClientMechanism) Start() ([]byte, error) {
	return []byte(m.AuthzID), nil
}

// Next is not called for EXTERNAL.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	return nil, fmt.Errorf("external: unexpected challenge")
}

// ServerMechanism implements EXTERNAL authentication for servers.
type ServerMechanism struct {
	auth auth.Authenticator
	done bool
}

// NewServerMechanism creates a new server-side EXTERNAL mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "EXTERNAL".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the client response.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	if m.done {
		return nil, true, fmt.Errorf("external: mechanism already completed")
	}
	m.done = true

	authzID := string(response)
	err := m.auth.Authenticate(context.Background(), Name, authzID, nil)
	return nil, true, err
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
