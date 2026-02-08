// Package xoauth2 implements the XOAUTH2 SASL mechanism used by Google.
package xoauth2

import (
	"context"
	"fmt"

	"github.com/meszmate/imap-go/auth"
)

// Mechanism name.
const Name = "XOAUTH2"

// ClientMechanism implements XOAUTH2 authentication for clients.
type ClientMechanism struct {
	Username    string
	AccessToken string
}

// Name returns "XOAUTH2".
func (m *ClientMechanism) Name() string { return Name }

// Start returns the initial response in XOAUTH2 format.
func (m *ClientMechanism) Start() ([]byte, error) {
	// Format: "user=" {User} "\x01auth=Bearer " {Access Token} "\x01\x01"
	ir := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", m.Username, m.AccessToken)
	return []byte(ir), nil
}

// Next handles error responses from the server.
func (m *ClientMechanism) Next(challenge []byte) ([]byte, error) {
	// Send empty response to acknowledge the error
	return []byte{}, nil
}

// ServerMechanism implements XOAUTH2 authentication for servers.
type ServerMechanism struct {
	auth auth.Authenticator
	done bool
}

// NewServerMechanism creates a new server-side XOAUTH2 mechanism.
func NewServerMechanism(authenticator auth.Authenticator) *ServerMechanism {
	return &ServerMechanism{auth: authenticator}
}

// Name returns "XOAUTH2".
func (m *ServerMechanism) Name() string { return Name }

// Next processes the client response.
func (m *ServerMechanism) Next(response []byte) ([]byte, bool, error) {
	if m.done {
		return nil, true, fmt.Errorf("xoauth2: mechanism already completed")
	}
	m.done = true

	// Parse the XOAUTH2 response
	username, token, err := parseXOAuth2(response)
	if err != nil {
		return nil, true, err
	}

	err = m.auth.Authenticate(context.Background(), Name, username, []byte(token))
	return nil, true, err
}

func parseXOAuth2(data []byte) (username, token string, err error) {
	s := string(data)
	var key string
	var i int
	for i < len(s) {
		eqIdx := -1
		for j := i; j < len(s); j++ {
			if s[j] == '=' {
				eqIdx = j
				break
			}
		}
		if eqIdx < 0 {
			break
		}
		key = s[i:eqIdx]
		valStart := eqIdx + 1
		valEnd := valStart
		for valEnd < len(s) && s[valEnd] != '\x01' {
			valEnd++
		}
		val := s[valStart:valEnd]

		switch key {
		case "user":
			username = val
		case "auth":
			// Strip "Bearer " prefix
			if len(val) > 7 && val[:7] == "Bearer " {
				token = val[7:]
			} else {
				token = val
			}
		}

		i = valEnd
		if i < len(s) && s[i] == '\x01' {
			i++
		}
	}

	if username == "" {
		return "", "", fmt.Errorf("xoauth2: missing username")
	}
	if token == "" {
		return "", "", fmt.Errorf("xoauth2: missing access token")
	}

	return username, token, nil
}

func init() {
	auth.DefaultRegistry.RegisterServer(Name, func(a auth.Authenticator) auth.ServerMechanism {
		return NewServerMechanism(a)
	})
}
