// Package acl implements the ACL extension (RFC 4314).
//
// ACL provides access control list commands for IMAP mailboxes. It allows
// administrators to grant and revoke rights for specific identifiers
// (users or groups) on mailboxes.
package acl

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionACL is an optional interface for sessions that support ACL commands.
type SessionACL interface {
	// SetACL sets the ACL rights for the given identifier on the mailbox.
	// The modifier is "+", "-", or "" (replace).
	SetACL(mailbox, identifier string, modifier string, rights imap.ACLRights) error

	// DeleteACL removes all rights for the given identifier on the mailbox.
	DeleteACL(mailbox, identifier string) error

	// GetACL returns the full ACL for the mailbox.
	GetACL(mailbox string) (*imap.ACLData, error)

	// ListRights returns the rights that can be granted to the identifier on the mailbox.
	ListRights(mailbox, identifier string) (*imap.ACLListRightsData, error)

	// MyRights returns the caller's rights on the mailbox.
	MyRights(mailbox string) (*imap.ACLMyRightsData, error)
}

// Extension implements the ACL extension (RFC 4314).
type Extension struct {
	extension.BaseExtension
}

// New creates a new ACL extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ACL",
			ExtCapabilities: []imap.Cap{imap.CapACL},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"SETACL":     server.CommandHandlerFunc(handleSetACL),
		"DELETEACL":  server.CommandHandlerFunc(handleDeleteACL),
		"GETACL":     server.CommandHandlerFunc(handleGetACL),
		"LISTRIGHTS": server.CommandHandlerFunc(handleListRights),
		"MYRIGHTS":   server.CommandHandlerFunc(handleMyRights),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionACL)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleSetACL handles the SETACL command.
//
// Command syntax: SETACL mailbox identifier rights
// The rights string may be prefixed with "+" or "-" to add/remove rights,
// or no prefix to replace all rights.
func handleSetACL(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionACL)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "SETACL not supported")
		return nil
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	identifier, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected rights")
		return nil
	}

	rightsStr, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected rights string")
		return nil
	}

	// Parse modifier prefix
	modifier := ""
	rights := rightsStr
	if len(rightsStr) > 0 {
		switch rightsStr[0] {
		case '+':
			modifier = "+"
			rights = rightsStr[1:]
		case '-':
			modifier = "-"
			rights = rightsStr[1:]
		}
	}

	if err := sess.SetACL(mailbox, identifier, modifier, imap.ACLRights(rights)); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("SETACL failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "SETACL completed")
	return nil
}

// handleDeleteACL handles the DELETEACL command.
//
// Command syntax: DELETEACL mailbox identifier
func handleDeleteACL(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionACL)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "DELETEACL not supported")
		return nil
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	identifier, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	if err := sess.DeleteACL(mailbox, identifier); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("DELETEACL failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "DELETEACL completed")
	return nil
}

// handleGetACL handles the GETACL command.
//
// Command syntax: GETACL mailbox
// Response:       * ACL mailbox identifier1 rights1 identifier2 rights2 ...
func handleGetACL(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionACL)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GETACL not supported")
		return nil
	}

	mailbox, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	data, err := sess.GetACL(mailbox)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GETACL failed: %v", err))
		return nil
	}

	// Write ACL response: * ACL mailbox id1 rights1 id2 rights2 ...
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("ACL").SP().MailboxName(data.Mailbox)
		for id, rights := range data.Rights {
			enc.SP().AString(id).SP().AString(string(rights))
		}
		enc.CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "GETACL completed")
	return nil
}

// handleListRights handles the LISTRIGHTS command.
//
// Command syntax: LISTRIGHTS mailbox identifier
// Response:       * LISTRIGHTS mailbox identifier required optional1 optional2 ...
func handleListRights(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionACL)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "LISTRIGHTS not supported")
		return nil
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	identifier, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected identifier")
		return nil
	}

	data, err := sess.ListRights(mailbox, identifier)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("LISTRIGHTS failed: %v", err))
		return nil
	}

	// Write LISTRIGHTS response
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("LISTRIGHTS").SP().
			MailboxName(data.Mailbox).SP().
			AString(data.Identifier).SP().
			AString(string(data.Required))
		for _, opt := range data.Optional {
			enc.SP().AString(string(opt))
		}
		enc.CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "LISTRIGHTS completed")
	return nil
}

// handleMyRights handles the MYRIGHTS command.
//
// Command syntax: MYRIGHTS mailbox
// Response:       * MYRIGHTS mailbox rights
func handleMyRights(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionACL)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "MYRIGHTS not supported")
		return nil
	}

	mailbox, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	data, err := sess.MyRights(mailbox)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("MYRIGHTS failed: %v", err))
		return nil
	}

	// Write MYRIGHTS response
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("MYRIGHTS").SP().
			MailboxName(data.Mailbox).SP().
			AString(string(data.Rights)).
			CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "MYRIGHTS completed")
	return nil
}
