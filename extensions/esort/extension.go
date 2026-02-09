// Package esort implements the ESORT extension (RFC 5267).
//
// ESORT extends the SORT command with ESEARCH-style return options
// (MIN, MAX, ALL, COUNT, PARTIAL), returning results in the ESEARCH
// response format instead of the traditional SORT response. This
// extension advertises the ESORT and CONTEXT=SORT capabilities and
// exposes a session interface for extended sort operations.
package esort

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionESort is the session interface for ESORT support.
// Implementations provide extended sort with ESEARCH-style return
// options, returning SearchData instead of SortData.
type SessionESort interface {
	SortExtended(kind server.NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the ESORT IMAP extension (RFC 5267).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ESORT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ESORT",
			ExtCapabilities: []imap.Cap{imap.CapESort, imap.CapContextSort},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionESort)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
