// Package contextsearch implements the CONTEXT=SEARCH extension (RFC 5267).
//
// CONTEXT=SEARCH adds the CONTEXT return option and UPDATE notifications
// to SEARCH commands, allowing clients to maintain live search results
// that are automatically updated as the mailbox changes. This extension
// advertises the CONTEXT=SEARCH capability and exposes a session interface
// for context-aware search operations with update support.
package contextsearch

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionContext is the session interface for CONTEXT=SEARCH support.
// Implementations provide persistent search contexts that deliver
// UPDATE notifications when the result set changes.
type SessionContext interface {
	SearchContext(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
	CancelSearchContext(tag string) error
}

// Extension implements the CONTEXT=SEARCH IMAP extension (RFC 5267).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CONTEXT=SEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CONTEXT=SEARCH",
			ExtCapabilities: []imap.Cap{imap.CapContextSearch},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionContext)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
