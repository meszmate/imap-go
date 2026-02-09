// Package uidplus implements the UIDPLUS extension (RFC 4315).
//
// UIDPLUS extends APPEND, COPY, and EXPUNGE with UID information in their
// responses (APPENDUID and COPYUID response codes) and adds the UID EXPUNGE
// command for expunging specific messages by UID.
//
// The core types already support UIDPLUS response data: CopyData contains
// UIDValidity, SourceUIDs, and DestUIDs for COPYUID; AppendData contains
// UIDValidity and UID for APPENDUID; and Session.Expunge accepts a *UIDSet
// for UID EXPUNGE. This extension exposes the session interface for backends
// that want to handle these operations separately and advertises the UIDPLUS
// capability.
package uidplus

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionUIDPlus is the session interface for UIDPLUS support.
// Backends implement this to provide UID information in COPY responses
// and to support UID EXPUNGE for removing specific messages by UID.
type SessionUIDPlus interface {
	// CopyUIDs copies messages to a destination mailbox and returns UID
	// mapping data (UIDValidity, SourceUIDs, DestUIDs) for the COPYUID
	// response code.
	CopyUIDs(numSet imap.NumSet, dest string) (*imap.CopyData, error)

	// ExpungeUIDs permanently removes messages with the specified UIDs.
	// Unlike a regular EXPUNGE, this only removes messages matching the
	// given UID set, rather than all messages flagged as \Deleted.
	ExpungeUIDs(w *server.ExpungeWriter, uids *imap.UIDSet) error
}

// Extension implements the UIDPLUS extension (RFC 4315).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UIDPLUS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UIDPLUS",
			ExtCapabilities: []imap.Cap{imap.CapUIDPlus},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// The core already handles APPEND, COPY, and EXPUNGE with UID support,
// so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// The core types already support all UIDPLUS fields (CopyData.UIDValidity,
// CopyData.SourceUIDs, CopyData.DestUIDs, AppendData.UIDValidity,
// AppendData.UID), so no wrapping is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionUIDPlus, indicating
// that sessions should implement this interface for full UIDPLUS support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUIDPlus)(nil)
}

// OnEnabled is called when a client enables UIDPLUS via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
