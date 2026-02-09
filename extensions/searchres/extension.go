// Package searchres implements the SEARCHRES extension (RFC 5182).
//
// SEARCHRES provides the ability to save search results and reference
// them later using the $ marker in subsequent commands. The core
// SearchOptions already supports ReturnSave and SearchCriteria has
// SaveResult; this extension advertises the capability and exposes
// a session interface for saving and retrieving search results.
package searchres

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionSearchRes is the session interface for SEARCHRES support.
// Implementations manage saved search result sets that can be
// referenced by subsequent commands using the $ marker.
type SessionSearchRes interface {
	SaveSearchResult(data *imap.SearchData) error
	GetSearchResult() (*imap.SeqSet, error)
}

// Extension implements the SEARCHRES IMAP extension (RFC 5182).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SEARCHRES extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SEARCHRES",
			ExtCapabilities: []imap.Cap{imap.CapSearchRes},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSearchRes)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
