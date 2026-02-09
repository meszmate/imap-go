// Package esearch implements the ESEARCH extension (RFC 4731).
//
// ESEARCH extends the SEARCH command with additional return options
// (MIN, MAX, ALL, COUNT, SAVE) and a new ESEARCH response format.
// The core SEARCH handler already supports SearchOptions with these
// return options; this extension advertises the capability and exposes
// a session interface for extended search operations.
package esearch

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionESearch is the session interface for ESEARCH support.
// Implementations provide extended search with return options such as
// MIN, MAX, ALL, COUNT, and SAVE.
type SessionESearch interface {
	SearchExtended(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the ESEARCH IMAP extension (RFC 4731).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ESEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ESEARCH",
			ExtCapabilities: []imap.Cap{imap.CapESearch},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionESearch)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
