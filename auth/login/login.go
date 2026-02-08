// Package login implements the LOGIN SASL mechanism (legacy).
package login

import (
	"context"
	"fmt"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "LOGIN"

// ClientMechanism implements LOGIN authentication for clients.
type ClientMechanism struct {
	Username string
	Password string
	step     int
}

// Name returns "LOGIN".
func (m *ClientMechanism) Name() string { return Name }

// Start returns nil (LOGIN has no initial response).
func (m *ClientMechanism) Start() ([]byte, error) {
	return nil, nil
}

// Next processes server challenges.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	switch m.step {
	case 0:
		m.step++
		return []byte(m.Username), nil
	case 1:
		m.step++
		return []byte(m.Password), nil
	default:
		return nil, fmt.Errorf("login: unexpected challenge")
	}
}

// ServerMechanism implements LOGIN authentication for servers.
type ServerMechanism struct {
	auth     auth.Authenticator
	step     int
	username string
}

// NewServerMechanism creates a new server-side LOGIN mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "LOGIN".
func (m *ServerMechanism) Name() string { return Name }

// Next processes client responses.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	switch m.step {
	case 0:
		m.step++
		// Send "Username:" challenge
		return []byte("Username:"), false, nil
	case 1:
		m.username = string(response)
		m.step++
		// Send "Password:" challenge
		return []byte("Password:"), false, nil
	case 2:
		m.step++
		password := string(response)
		err := m.auth.Authenticate(context.Background(), Name, m.username, []byte(password))
		return nil, true, err
	default:
		return nil, true, fmt.Errorf("login: unexpected response")
	}
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
