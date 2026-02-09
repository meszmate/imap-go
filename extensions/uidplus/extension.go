// Package uidplus implements the UIDPLUS extension (RFC 4315).
//
// UIDPLUS extends APPEND, COPY, and EXPUNGE with UID information in their
// responses (APPENDUID and COPYUID response codes) and adds the UID EXPUNGE
// command for expunging specific messages by UID.
//
// The core types already support UIDPLUS response data: CopyData contains
// UIDValidity, SourceUIDs, and DestUIDs for COPYUID; AppendData contains
// UIDValidity and UID for APPENDUID; and Session.Expunge accepts a *UIDSet
// for UID EXPUNGE. This extension routes COPY to SessionUIDPlus.CopyUIDs()
// and UID EXPUNGE to SessionUIDPlus.ExpungeUIDs() when the backend implements
// the SessionUIDPlus interface.
package uidplus

import (
	"fmt"

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

// WrapHandler wraps existing command handlers to route to SessionUIDPlus
// methods when the backend implements the interface.
// It wraps COPY (to call CopyUIDs) and EXPUNGE (to call ExpungeUIDs for
// UID EXPUNGE).
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	h, ok := handler.(server.CommandHandlerFunc)
	if !ok {
		ch, ok2 := handler.(server.CommandHandler)
		if !ok2 {
			return nil
		}
		h = ch.Handle
	}

	switch name {
	case "COPY":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDPlusCopy(ctx, h)
		})
	case "EXPUNGE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDPlusExpunge(ctx, h)
		})
	}
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

// handleUIDPlusCopy wraps the COPY command to route to SessionUIDPlus.CopyUIDs()
// when the backend implements the interface, and writes the COPYUID response code.
func handleUIDPlusCopy(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	// Read sequence set
	seqSetStr, err := ctx.Decoder.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid sequence set")
	}

	var numSet imap.NumSet
	if ctx.NumKind == server.NumKindUID {
		uidSet, err := imap.ParseUIDSet(seqSetStr)
		if err != nil {
			return imap.ErrBad("invalid UID set")
		}
		numSet = uidSet
	} else {
		seqSet, err := imap.ParseSeqSet(seqSetStr)
		if err != nil {
			return imap.ErrBad("invalid sequence set")
		}
		numSet = seqSet
	}

	if err := ctx.Decoder.ReadSP(); err != nil {
		return imap.ErrBad("missing destination mailbox")
	}

	dest, err := ctx.Decoder.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid destination mailbox")
	}

	// Route to SessionUIDPlus.CopyUIDs if available, else Session.Copy
	var data *imap.CopyData
	if sess, ok := ctx.Session.(SessionUIDPlus); ok {
		data, err = sess.CopyUIDs(numSet, dest)
	} else {
		data, err = ctx.Session.Copy(numSet, dest)
	}
	if err != nil {
		return err
	}

	// Write tagged OK, optionally with COPYUID response code
	if data != nil && data.UIDValidity > 0 {
		code := fmt.Sprintf("COPYUID %d %s %s",
			data.UIDValidity,
			data.SourceUIDs.String(),
			data.DestUIDs.String())
		ctx.Conn.WriteOKCode(ctx.Tag, code, "COPY completed")
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "COPY completed")
	}

	return nil
}

// handleUIDPlusExpunge wraps the EXPUNGE command to route UID EXPUNGE to
// SessionUIDPlus.ExpungeUIDs() when the backend implements the interface.
func handleUIDPlusExpunge(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	// For UID EXPUNGE, parse the UID set
	var uids *imap.UIDSet
	if ctx.NumKind == server.NumKindUID && ctx.Decoder != nil {
		uidStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid UID set")
		}
		uidSet, err := imap.ParseUIDSet(uidStr)
		if err != nil {
			return imap.ErrBad("invalid UID set")
		}
		uids = uidSet
	}

	w := server.NewExpungeWriter(ctx.Conn.Encoder())

	// Route UID EXPUNGE to SessionUIDPlus.ExpungeUIDs if available
	if uids != nil {
		if sess, ok := ctx.Session.(SessionUIDPlus); ok {
			if err := sess.ExpungeUIDs(w, uids); err != nil {
				return err
			}
		} else {
			if err := ctx.Session.Expunge(w, uids); err != nil {
				return err
			}
		}
	} else {
		if err := ctx.Session.Expunge(w, nil); err != nil {
			return err
		}
	}

	ctx.Conn.WriteOK(ctx.Tag, "EXPUNGE completed")
	return nil
}
