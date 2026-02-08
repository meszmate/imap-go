// Package oauthbearer implements the OAUTHBEARER SASL mechanism (RFC 7628).
package oauthbearer

import (
	"context"
	"fmt"
	"strings"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "OAUTHBEARER"

// ClientMechanism implements OAUTHBEARER authentication for clients.
type ClientMechanism struct {
	Username    string
	AccessToken string
	Host        string
	Port        string
}

// Name returns "OAUTHBEARER".
func (m *ClientMechanism) Name() string { return Name }

// Start returns the initial client response per RFC 7628.
func (m *ClientMechanism) Start() ([]byte, error) {
	// GS2 header: n,,
	// Then key-value pairs separated by \x01
	var b strings.Builder
	b.WriteString("n,a=")
	b.WriteString(m.Username)
	b.WriteString(",")
	b.WriteByte(0x01)
	if m.Host != "" {
		b.WriteString("host=")
		b.WriteString(m.Host)
		b.WriteByte(0x01)
	}
	if m.Port != "" {
		b.WriteString("port=")
		b.WriteString(m.Port)
		b.WriteByte(0x01)
	}
	b.WriteString("auth=Bearer ")
	b.WriteString(m.AccessToken)
	b.WriteByte(0x01)
	b.WriteByte(0x01)
	return []byte(b.String()), nil
}

// Next handles error responses from the server.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	// Acknowledge error
	return []byte{0x01}, nil
}

// ServerMechanism implements OAUTHBEARER authentication for servers.
type ServerMechanism struct {
	auth auth.Authenticator
	done bool
}

// NewServerMechanism creates a new server-side OAUTHBEARER mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "OAUTHBEARER".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the client response.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	if m.done {
		return nil, true, fmt.Errorf("oauthbearer: mechanism already completed")
	}
	m.done = true

	username, token, err := parseOAuthBearer(response)
	if err != nil {
		return nil, true, err
	}

	err = m.auth.Authenticate(context.Background(), Name, username, []byte(token))
	return nil, true, err
}

func parseOAuthBearer(data []byte) (username, token string, err error) {
	s := string(data)

	// Parse GS2 header: n,a=<user>,\x01...
	if !strings.HasPrefix(s, "n,") {
		return "", "", fmt.Errorf("oauthbearer: invalid GS2 header")
	}
	s = s[2:]

	commaIdx := strings.IndexByte(s, ',')
	if commaIdx < 0 {
		return "", "", fmt.Errorf("oauthbearer: invalid format")
	}
	authzPart := s[:commaIdx]
	if strings.HasPrefix(authzPart, "a=") {
		username = authzPart[2:]
	}
	s = s[commaIdx+1:]

	// Parse key-value pairs separated by \x01
	parts := strings.Split(s, "\x01")
	for _, part := range parts {
		if strings.HasPrefix(part, "auth=Bearer ") {
			token = part[len("auth=Bearer "):]
		} else if strings.HasPrefix(part, "auth=") {
			token = part[5:]
		}
	}

	if username == "" {
		return "", "", fmt.Errorf("oauthbearer: missing username")
	}
	if token == "" {
		return "", "", fmt.Errorf("oauthbearer: missing access token")
	}

	return username, token, nil
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
