// Package uidonly implements the UIDONLY extension (RFC 9586).
//
// UIDONLY enables a UID-only mode where the server stops using message
// sequence numbers in untagged responses and instead uses UIDs exclusively.
// It is activated via the ENABLE command (ENABLE UIDONLY), after which all
// FETCH, STORE, SEARCH, and EXPUNGE responses use UIDs instead of sequence
// numbers.
//
// This extension exposes the session interface for backends to be notified
// when UID-only mode is activated and advertises the UIDONLY capability.
package uidonly

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionUIDOnly is the session interface for UIDONLY support.
// Backends implement this to be notified when a client enables UID-only
// mode, allowing the session to adjust its response format to use UIDs
// exclusively instead of message sequence numbers.
type SessionUIDOnly interface {
	// EnableUIDOnly is called when a client enables UID-only mode via
	// ENABLE UIDONLY. After this call, the session should use UIDs
	// instead of sequence numbers in all untagged responses including
	// FETCH, STORE, SEARCH, and EXPUNGE.
	EnableUIDOnly() error
}

// Extension implements the UIDONLY extension (RFC 9586).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UIDONLY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UIDONLY",
			ExtCapabilities: []imap.Cap{imap.CapUIDOnly},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// UIDONLY is activated via the ENABLE command, which is handled
// separately, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// UID-only mode activation is handled through the ENABLE command and
// session interface, so no wrapping is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionUIDOnly, indicating
// that sessions should implement this interface for full UIDONLY support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUIDOnly)(nil)
}

// OnEnabled is called when a client enables UIDONLY via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
