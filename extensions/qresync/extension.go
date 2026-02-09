// Package qresync implements the QRESYNC extension (RFC 7162).
//
// QRESYNC (Quick Resynchronization) allows a client to efficiently
// resynchronize its local cache with the server by providing known UIDs
// and modification sequences during SELECT. It depends on CONDSTORE.
//
// The core SelectOptions already supports the QResync field with all necessary
// sub-fields (UIDValidity, ModSeq, KnownUIDs, SeqMatch). This extension
// exposes the session interface for backends that want to handle QRESYNC
// SELECT operations separately and advertises the QRESYNC capability.
package qresync

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionQResync is the session interface for QRESYNC support.
// Backends implement this to handle SELECT with QRESYNC parameters,
// which allows efficient mailbox resynchronization by providing known
// UIDs, modification sequences, and optional sequence-to-UID mappings.
type SessionQResync interface {
	// SelectQResync opens a mailbox with QRESYNC parameters.
	// The options.QResync field contains UIDValidity, ModSeq, KnownUIDs,
	// and optional SeqMatch data for efficient resynchronization.
	SelectQResync(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error)
}

// Extension implements the QRESYNC extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

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
// adding new commands, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// The core SelectOptions already supports the QResync field, so no wrapping
// is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionQResync, indicating
// that sessions should implement this interface for full QRESYNC support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionQResync)(nil)
}

// OnEnabled is called when a client enables QRESYNC via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
