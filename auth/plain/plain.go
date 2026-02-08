// Package plain implements the PLAIN SASL mechanism (RFC 4616).
package plain

import (
	"bytes"
	"context"
	"fmt"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "PLAIN"

// ClientMechanism implements PLAIN authentication for clients.
type ClientMechanism struct {
	// AuthzID is the authorization identity (usually empty).
	AuthzID string
	// Username is the authentication identity.
	Username string
	// Password is the password.
	Password string
}

// Name returns "PLAIN".
func (m *ClientMechanism) Name() string { return Name }

// Start returns the initial response: authzid\0authcid\0passwd.
func (m *ClientMechanism) Start() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(m.AuthzID)
	buf.WriteByte(0)
	buf.WriteString(m.Username)
	buf.WriteByte(0)
	buf.WriteString(m.Password)
	return buf.Bytes(), nil
}

// Next is not called for PLAIN since the initial response contains everything.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	return nil, fmt.Errorf("plain: unexpected challenge")
}

// ServerMechanism implements PLAIN authentication for servers.
type ServerMechanism struct {
	auth auth.Authenticator
	done bool
}

// NewServerMechanism creates a new server-side PLAIN mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "PLAIN".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the client's initial response.
// The response format is: [authzid] \0 authcid \0 passwd
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	if m.done {
		return nil, true, fmt.Errorf("plain: mechanism already completed")
	}
	m.done = true

	parts := bytes.SplitN(response, []byte{0}, 3)
	if len(parts) != 3 {
		return nil, true, fmt.Errorf("plain: invalid response format")
	}

	authzID := string(parts[0])
	username := string(parts[1])
	password := string(parts[2])

	if authzID == "" {
		authzID = username
	}

	_ = authzID // authzID handling is for the authenticator

	err := m.auth.Authenticate(context.Background(), Name, username, []byte(password))
	return nil, true, err
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
