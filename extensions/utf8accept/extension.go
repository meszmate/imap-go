// Package utf8accept implements the UTF8=ACCEPT extension (RFC 6855).
//
// UTF8=ACCEPT allows the IMAP server to accept and send UTF-8 encoded
// strings in mailbox names, message headers, and other protocol elements.
// It is activated via the ENABLE command (ENABLE UTF8=ACCEPT), after which
// the server operates in UTF-8 mode for the connection.
//
// This extension exposes the session interface for backends to be notified
// when UTF-8 mode is activated and advertises the UTF8=ACCEPT capability.
package utf8accept

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionUTF8Accept is the session interface for UTF8=ACCEPT support.
// Backends implement this to be notified when a client enables UTF-8 mode,
// allowing the session to adjust its behavior for UTF-8 encoded data.
type SessionUTF8Accept interface {
	// EnableUTF8 is called when a client enables UTF-8 mode via
	// ENABLE UTF8=ACCEPT. After this call, the session should accept
	// and produce UTF-8 encoded strings in mailbox names, headers,
	// and other protocol elements.
	EnableUTF8() error
}

// Extension implements the UTF8=ACCEPT extension (RFC 6855).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UTF8=ACCEPT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UTF8=ACCEPT",
			ExtCapabilities: []imap.Cap{imap.CapUTF8Accept},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// UTF8=ACCEPT is activated via the ENABLE command, which is handled
// separately, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// UTF-8 mode activation is handled through the ENABLE command and
// session interface, so no wrapping is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionUTF8Accept,
// indicating that sessions should implement this interface for full
// UTF8=ACCEPT support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUTF8Accept)(nil)
}

// OnEnabled is called when a client enables UTF8=ACCEPT via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
