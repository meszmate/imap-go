// Package multisearch implements the MULTISEARCH extension (RFC 7377).
//
// MULTISEARCH extends the SEARCH command to allow searching across multiple
// mailboxes simultaneously. The core SEARCH behavior is handled by the
// session backend; this extension advertises the capability and exposes the
// SessionMultiSearch interface for backends that want to support multi-mailbox
// search operations.
package multisearch

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionMultiSearch is an optional interface for sessions that support
// the MULTISEARCH extension (RFC 7377).
type SessionMultiSearch interface {
	// MultiSearch performs a search across the specified mailboxes.
	// If mailboxes is empty, the server should search all accessible mailboxes.
	MultiSearch(mailboxes []string, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the MULTISEARCH IMAP extension (RFC 7377).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new MULTISEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "MULTISEARCH",
			ExtCapabilities: []imap.Cap{imap.CapMultiSearch},
		},
	}
}

// CommandHandlers returns nil because MULTISEARCH extends the SEARCH command
// behavior which is handled by the session backend.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil because multi-mailbox search behavior is delegated
// to the session backend via the SessionMultiSearch interface.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionMultiSearch interface that sessions may
// implement to support multi-mailbox search operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionMultiSearch)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
