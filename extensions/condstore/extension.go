// Package condstore implements the CONDSTORE extension (RFC 7162).
//
// CONDSTORE adds conditional STORE operations and per-message modification
// sequence numbers (MODSEQ). It modifies FETCH, STORE, SELECT, and SEARCH
// to handle MODSEQ values.
package condstore

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the CONDSTORE extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

// New creates a new CONDSTORE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CONDSTORE",
			ExtCapabilities: []imap.Cap{imap.CapCondStore},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// CONDSTORE modifies existing commands (FETCH, STORE, SELECT, SEARCH) rather
// than adding new ones, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// In a full implementation this would wrap FETCH, STORE, SELECT, and SEARCH
// to handle MODSEQ parameters and responses.
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
