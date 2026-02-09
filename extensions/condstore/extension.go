// Package condstore implements the CONDSTORE extension (RFC 7162).
//
// CONDSTORE adds conditional STORE operations and per-message modification
// sequence numbers (MODSEQ). It modifies FETCH, STORE, SELECT, and SEARCH
// to handle MODSEQ values.
//
// The core types already support CONDSTORE fields: FetchOptions.ModSeq,
// FetchOptions.ChangedSince, StoreOptions.UnchangedSince, SelectOptions.CondStore,
// and SearchCriteria.ModSeq. This extension exposes the session interface for
// backends that want to handle conditional store operations separately and
// advertises the CONDSTORE capability.
package condstore

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionCondStore is the session interface for CONDSTORE support.
// Backends implement this to handle conditional STORE operations that use
// the UNCHANGEDSINCE modifier, returning per-message MODSEQ values.
type SessionCondStore interface {
	// StoreConditional stores flags on messages conditionally based on MODSEQ.
	// The options.UnchangedSince field specifies the MODSEQ threshold;
	// messages modified since that value must not be updated.
	StoreConditional(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error
}

// Extension implements the CONDSTORE extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

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
// The core types already support all CONDSTORE fields (FetchOptions.ModSeq,
// FetchOptions.ChangedSince, StoreOptions.UnchangedSince, SelectOptions.CondStore,
// SearchCriteria.ModSeq), so no wrapping is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionCondStore, indicating
// that sessions should implement this interface for full CONDSTORE support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionCondStore)(nil)
}

// OnEnabled is called when a client enables CONDSTORE via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
