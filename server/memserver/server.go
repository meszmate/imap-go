// Package memserver provides a complete in-memory IMAP backend for testing.
//
// It implements the server.Session interface with all standard IMAP operations
// backed by in-memory data structures. This is useful for testing IMAP clients
// and server infrastructure without requiring a real mail store.
//
// Usage:
//
//	ms := memserver.New()
//	ms.AddUser("user", "password")
//	srv := ms.NewServer(server.WithAllowInsecureAuth(true))
//	srv.ListenAndServe(":143")
package memserver

import (
	"sync"

	"github.com/meszmate/imap-go/server"
)

// MemServer is an in-memory IMAP backend. It stores user credentials and
// mailbox data entirely in memory.
type MemServer struct {
	mu       sync.RWMutex
	users    map[string]string    // username -> password
	userData map[string]*UserData // username -> mailbox data
}

// New creates a new MemServer.
func New() *MemServer {
	return &MemServer{
		users:    make(map[string]string),
		userData: make(map[string]*UserData),
	}
}

// AddUser adds a user with the given username and password.
// If the user already exists, the password is updated.
// Each new user gets a default INBOX mailbox.
func (ms *MemServer) AddUser(username, password string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.users[username] = password
	if _, exists := ms.userData[username]; !exists {
		ms.userData[username] = NewUserData()
	}
}

// RemoveUser removes a user and all associated data.
func (ms *MemServer) RemoveUser(username string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.users, username)
	delete(ms.userData, username)
}

// GetUserData returns the UserData for a user, or nil if the user doesn't exist.
// This is useful for tests that want to pre-populate mailbox data.
func (ms *MemServer) GetUserData(username string) *UserData {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.userData[username]
}

// NewSession creates a new Session for a connection. This is the callback
// used by the server to create sessions for new connections.
func (ms *MemServer) NewSession(conn *server.Conn) (server.Session, error) {
	return &Session{
		srv: ms,
	}, nil
}

// NewServer creates a new server.Server configured to use this MemServer
// as its backend. Additional server options can be passed.
func (ms *MemServer) NewServer(opts ...server.Option) *server.Server {
	allOpts := []server.Option{
		server.WithNewSession(ms.NewSession),
		server.WithAllowInsecureAuth(true),
	}
	allOpts = append(allOpts, opts...)

	return server.New(allOpts...)
}
