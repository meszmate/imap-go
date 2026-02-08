// Package notify implements the NOTIFY extension (RFC 5465).
//
// NOTIFY allows a client to request that the server send unsolicited
// notifications about changes to specified mailboxes. This enables
// real-time monitoring of multiple mailboxes without requiring separate
// IDLE connections for each one.
//
// This is a capability-only registration; the full NOTIFY command
// implementation will be added in a future update.
package notify

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the NOTIFY extension (RFC 5465).
type Extension struct {
	extension.BaseExtension
}

// New creates a new NOTIFY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "NOTIFY",
			ExtCapabilities: []imap.Cap{imap.CapNotify},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// A full implementation would add the NOTIFY command handler here.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return nil
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
