// Package specialuse implements the SPECIAL-USE IMAP extension (RFC 6154).
//
// SPECIAL-USE allows the server to advertise special-use attributes
// (such as \Drafts, \Sent, \Trash, \Junk, \All, \Archive, \Flagged)
// on mailboxes via LIST responses, and allows clients to create mailboxes
// with specific special-use attributes. The core LIST command already
// handles selection and return of special-use attributes via ListOptions,
// and the core CREATE command already passes CreateOptions with SpecialUse;
// this extension advertises the SPECIAL-USE and CREATE-SPECIAL-USE
// capabilities, wraps the CREATE handler to parse the USE parameter,
// and exposes a session interface.
package specialuse

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionSpecialUse is an optional interface for sessions that support
// the SPECIAL-USE extension. Backends implementing this interface can
// handle special-use attribute filtering in LIST and creation of
// mailboxes with special-use attributes.
type SessionSpecialUse interface {
	// ListSpecialUse lists mailboxes with special-use attribute handling.
	ListSpecialUse(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error

	// CreateSpecialUse creates a mailbox with a special-use attribute.
	CreateSpecialUse(mailbox string, options *imap.CreateOptions) error
}

// Extension implements the SPECIAL-USE IMAP extension (RFC 6154).
// SPECIAL-USE allows the server to advertise special-use attributes
// on mailboxes via LIST responses and supports creating mailboxes with
// special-use attributes. The core commands already handle these via
// ListOptions and CreateOptions; this extension advertises the capabilities
// and exposes a session interface.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SPECIAL-USE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SPECIAL-USE",
			ExtCapabilities: []imap.Cap{imap.CapSpecialUse, imap.CapCreateSpecialUse},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps the CREATE command to parse the optional USE parameter
// per RFC 6154 §3: CREATE mailbox-name SP (USE (special-use-attr))
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
	case "CREATE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleSpecialUseCreate(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the SessionSpecialUse interface that sessions
// may implement to support special-use mailbox attributes.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSpecialUse)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleSpecialUseCreate wraps the CREATE command to parse the optional
// USE parameter (RFC 6154 §3).
// Wire format: CREATE mailbox-name [SP (USE (special-use-attr))]
func handleSpecialUseCreate(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing mailbox name")
	}

	mailbox, err := ctx.Decoder.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid mailbox name")
	}

	options := &imap.CreateOptions{}

	// Try to read SP followed by (USE (\Attr))
	if err := ctx.Decoder.ReadSP(); err == nil {
		// Read opening paren
		if err := ctx.Decoder.ExpectByte('('); err != nil {
			return imap.ErrBad("expected '(' after SP")
		}

		// Read "USE" atom
		atom, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("expected USE keyword")
		}
		if !strings.EqualFold(atom, "USE") {
			return imap.ErrBad("expected USE keyword")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("expected SP after USE")
		}

		// Read inner paren list: (\Attr)
		if err := ctx.Decoder.ExpectByte('('); err != nil {
			return imap.ErrBad("expected '(' before attribute")
		}

		// Read backslash + atom for the attribute
		if err := ctx.Decoder.ExpectByte('\\'); err != nil {
			return imap.ErrBad("expected '\\' for special-use attribute")
		}
		attrName, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid special-use attribute")
		}
		options.SpecialUse = imap.MailboxAttr("\\" + attrName)

		// Close inner paren
		if err := ctx.Decoder.ExpectByte(')'); err != nil {
			return imap.ErrBad("expected ')' after attribute")
		}

		// Close outer paren
		if err := ctx.Decoder.ExpectByte(')'); err != nil {
			return imap.ErrBad("expected ')' after USE list")
		}
	}

	// Route to SessionSpecialUse if available
	if options.SpecialUse != "" {
		if sess, ok := ctx.Session.(SessionSpecialUse); ok {
			if err := sess.CreateSpecialUse(mailbox, options); err != nil {
				return err
			}
			ctx.Conn.WriteOK(ctx.Tag, "CREATE completed")
			return nil
		}
	}

	if err := ctx.Session.Create(mailbox, options); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "CREATE completed")
	return nil
}
