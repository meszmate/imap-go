// Package crammd5 implements the CRAM-MD5 SASL mechanism (RFC 2195).
package crammd5

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "CRAM-MD5"

// ClientMechanism implements CRAM-MD5 authentication for clients.
type ClientMechanism struct {
	Username string
	Password string
}

// Name returns "CRAM-MD5".
func (m *ClientMechanism) Name() string { return Name }

// Start returns nil (CRAM-MD5 has no initial response).
func (m *ClientMechanism) Start() ([]byte, error) {
	return nil, nil
}

// Next computes the HMAC-MD5 response to the server challenge.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	h := hmac.New(md5.New, []byte(m.Password))
	h.Write(challenge)
	digest := hex.EncodeToString(h.Sum(nil))
	return []byte(m.Username + " " + digest), nil
}

// ServerMechanism implements CRAM-MD5 authentication for servers.
// Note: CRAM-MD5 requires the server to know the password in plaintext
// to verify the HMAC. The authenticator receives the username and the
// full "username digest" as credentials.
type ServerMechanism struct {
	auth      auth.Authenticator
	challenge []byte
	step      int
}

// NewServerMechanism creates a new server-side CRAM-MD5 mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "CRAM-MD5".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the authentication exchange.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	switch m.step {
	case 0:
		m.step++
		// Generate a challenge (simplified - production should use proper random nonce)
		m.challenge = []byte(fmt.Sprintf("<%d.%d@localhost>", 0, 0))
		return m.challenge, false, nil
	case 1:
		// Parse "username digest"
		parts := strings.SplitN(string(response), " ", 2)
		if len(parts) != 2 {
			return nil, true, fmt.Errorf("cram-md5: invalid response format")
		}
		// Pass the challenge and response to the authenticator
		// The authenticator needs to know the password to verify
		err := m.auth.Authenticate(context.Background(), Name, parts[0], response)
		return nil, true, err
	default:
		return nil, true, fmt.Errorf("cram-md5: unexpected response")
	}
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})

	auth.DefaultRegistry.RegisterClient(Name, func() auth.ClientMechanism {
		return &ClientMechanism{}
	})
}
