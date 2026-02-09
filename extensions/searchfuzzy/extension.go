// Package searchfuzzy implements the SEARCH=FUZZY extension (RFC 6203).
//
// SEARCH=FUZZY adds support for fuzzy matching in SEARCH commands.
// When enabled, the server may use approximate matching for search
// criteria. The core SearchCriteria already contains the Fuzzy flag;
// this extension advertises the capability and exposes a session
// interface for fuzzy search operations.
package searchfuzzy

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionSearchFuzzy is the session interface for SEARCH=FUZZY support.
// Implementations provide fuzzy (approximate) matching for search criteria.
type SessionSearchFuzzy interface {
	SearchFuzzy(criteria *imap.SearchCriteria) (*imap.SearchData, error)
}

// Extension implements the SEARCH=FUZZY IMAP extension (RFC 6203).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SEARCH=FUZZY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SEARCH=FUZZY",
			ExtCapabilities: []imap.Cap{imap.CapSearchFuzzy},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSearchFuzzy)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
