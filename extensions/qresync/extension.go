// Package qresync implements the QRESYNC extension (RFC 7162).
//
// QRESYNC (Quick Resynchronization) allows a client to efficiently
// resynchronize its local cache with the server by providing known UIDs
// and modification sequences. It depends on CONDSTORE.
package qresync

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the QRESYNC extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

// New creates a new QRESYNC extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "QRESYNC",
			ExtCapabilities: []imap.Cap{imap.CapQResync},
			ExtDependencies: []string{"CONDSTORE"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// QRESYNC modifies the SELECT command with additional parameters rather than
// adding new commands. A full implementation would wrap SELECT.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// In a full implementation this would wrap SELECT to handle QRESYNC parameters
// including known UIDs and known sequence sets for efficient resynchronization.
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
