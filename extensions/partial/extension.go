// Package partial implements the PARTIAL extension (RFC 9394).
//
// PARTIAL adds support for requesting partial search and sort results,
// allowing clients to paginate through large result sets. The core
// SearchOptions already contains ReturnPartial and SearchData has
// Partial; this extension advertises the capability and exposes a
// session interface for partial result operations.
package partial

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionPartial is the session interface for PARTIAL support.
// Implementations provide paginated search results via the PARTIAL
// return option.
type SessionPartial interface {
	SearchPartial(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the PARTIAL IMAP extension (RFC 9394).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new PARTIAL extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "PARTIAL",
			ExtCapabilities: []imap.Cap{imap.CapPartial},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionPartial)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
