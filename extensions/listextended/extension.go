// Package listextended implements the LIST-EXTENDED IMAP extension (RFC 5258).
//
// LIST-EXTENDED adds selection and return options to the LIST command,
// enabling features like subscribed-only listing, children info, and
// special-use attribute filtering.
package listextended

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionListExtended is an optional interface for sessions that support
// the LIST-EXTENDED extension. Backends implementing this interface can
// handle selection options (SUBSCRIBED, REMOTE, RECURSIVEMATCH) and
// return options (SUBSCRIBED, CHILDREN) in the LIST command.
type SessionListExtended interface {
	ListExtended(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
}

// Extension implements the LIST-EXTENDED IMAP extension (RFC 5258).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-EXTENDED extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-EXTENDED",
			ExtCapabilities: []imap.Cap{imap.CapListExtended},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps the LIST command handler to support extended syntax.
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
	case "LIST":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleListExtended(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the SessionListExtended interface that sessions
// may implement to support extended LIST operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionListExtended)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleListExtended wraps the LIST command to parse extended syntax.
func handleListExtended(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder
	options := &imap.ListOptions{}
	isExtended := false

	// Peek to check if first byte is '(' indicating selection options
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("missing arguments")
	}

	var ref string
	var patterns []string

	if b == '(' {
		// Extended syntax with selection options
		isExtended = true
		if err := parseSelectionOptions(dec, options); err != nil {
			return err
		}

		// Validate: RECURSIVEMATCH requires another selection option
		if options.SelectRecursiveMatch && !options.SelectSubscribed && !options.SelectRemote && !options.SelectSpecialUse {
			return imap.ErrBad("RECURSIVEMATCH requires another selection option")
		}

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing reference name")
		}

		// Read reference name
		ref, err = dec.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid reference name")
		}

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing mailbox pattern")
		}

		// Parse patterns (single or parenthesized list)
		patterns, err = readPatterns(dec)
		if err != nil {
			return err
		}
	} else {
		// Basic or basic-with-RETURN syntax: ref SP pattern
		ref, err = dec.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid reference name")
		}

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing mailbox pattern")
		}

		patterns, err = readPatterns(dec)
		if err != nil {
			return err
		}
	}

	// Check for RETURN options
	if err := dec.ReadSP(); err == nil {
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid argument after pattern")
		}
		if !strings.EqualFold(atom, "RETURN") {
			return imap.ErrBad("expected RETURN, got " + atom)
		}
		isExtended = true
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing RETURN options")
		}
		if err := parseReturnOptions(dec, options); err != nil {
			return err
		}
	}

	// Route to session
	w := server.NewListWriter(ctx.Conn.Encoder())
	if isExtended {
		if sess, ok := ctx.Session.(SessionListExtended); ok {
			if err := sess.ListExtended(w, ref, patterns, options); err != nil {
				return err
			}
		} else {
			if err := ctx.Session.List(w, ref, patterns, options); err != nil {
				return err
			}
		}
	} else {
		if err := ctx.Session.List(w, ref, patterns, options); err != nil {
			return err
		}
	}

	ctx.Conn.WriteOK(ctx.Tag, "LIST completed")
	return nil
}

// readPatterns reads either a single pattern or a parenthesized list of patterns.
func readPatterns(dec *wire.Decoder) ([]string, error) {
	b, err := dec.PeekByte()
	if err != nil {
		return nil, imap.ErrBad("missing mailbox pattern")
	}

	if b == '(' {
		// Multiple patterns in parenthesized list
		var patterns []string
		if err := dec.ExpectByte('('); err != nil {
			return nil, imap.ErrBad("expected '(' for pattern list")
		}

		for {
			b, err := dec.PeekByte()
			if err != nil {
				return nil, imap.ErrBad("unexpected end in pattern list")
			}
			if b == ')' {
				_ = dec.ExpectByte(')')
				break
			}
			if len(patterns) > 0 {
				if err := dec.ReadSP(); err != nil {
					return nil, imap.ErrBad("expected SP between patterns")
				}
			}
			p, err := dec.ReadAString()
			if err != nil {
				return nil, imap.ErrBad("invalid pattern")
			}
			patterns = append(patterns, p)
		}
		if len(patterns) == 0 {
			return nil, imap.ErrBad("empty pattern list")
		}
		return patterns, nil
	}

	// Single pattern
	p, err := dec.ReadAString()
	if err != nil {
		return nil, imap.ErrBad("invalid mailbox pattern")
	}
	return []string{p}, nil
}

// parseSelectionOptions parses a parenthesized list of selection options.
func parseSelectionOptions(dec *wire.Decoder, options *imap.ListOptions) error {
	if err := dec.ExpectByte('('); err != nil {
		return imap.ErrBad("expected '(' for selection options")
	}

	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end in selection options")
	}
	if b == ')' {
		_ = dec.ExpectByte(')')
		return nil
	}

	for {
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid selection option")
		}
		switch strings.ToUpper(atom) {
		case "SUBSCRIBED":
			options.SelectSubscribed = true
		case "REMOTE":
			options.SelectRemote = true
		case "RECURSIVEMATCH":
			options.SelectRecursiveMatch = true
		case "SPECIAL-USE":
			options.SelectSpecialUse = true
		default:
			return imap.ErrBad("unknown selection option: " + atom)
		}

		b, err := dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end in selection options")
		}
		if b == ')' {
			_ = dec.ExpectByte(')')
			return nil
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("expected SP between selection options")
		}
	}
}

// parseReturnOptions parses a parenthesized list of return options.
func parseReturnOptions(dec *wire.Decoder, options *imap.ListOptions) error {
	if err := dec.ExpectByte('('); err != nil {
		return imap.ErrBad("expected '(' for RETURN options")
	}

	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end in RETURN options")
	}
	if b == ')' {
		_ = dec.ExpectByte(')')
		return nil
	}

	for {
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid RETURN option")
		}
		switch strings.ToUpper(atom) {
		case "SUBSCRIBED":
			options.ReturnSubscribed = true
		case "CHILDREN":
			options.ReturnChildren = true
		case "SPECIAL-USE":
			options.ReturnSpecialUse = true
		case "MYRIGHTS":
			options.ReturnMyRights = true
		case "STATUS":
			// STATUS is followed by SP and a parenthesized list of status items
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing STATUS items")
			}
			statusOpts := &imap.StatusOptions{}
			if err := dec.ReadList(func() error {
				item, err := dec.ReadAtom()
				if err != nil {
					return err
				}
				switch strings.ToUpper(item) {
				case "MESSAGES":
					statusOpts.NumMessages = true
				case "UIDNEXT":
					statusOpts.UIDNext = true
				case "UIDVALIDITY":
					statusOpts.UIDValidity = true
				case "UNSEEN":
					statusOpts.NumUnseen = true
				case "RECENT":
					statusOpts.NumRecent = true
				case "SIZE":
					statusOpts.Size = true
				case "HIGHESTMODSEQ":
					statusOpts.HighestModSeq = true
				}
				return nil
			}); err != nil {
				return imap.ErrBad("invalid STATUS items list")
			}
			options.ReturnStatus = statusOpts
		default:
			return imap.ErrBad("unknown RETURN option: " + atom)
		}

		b, err := dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end in RETURN options")
		}
		if b == ')' {
			_ = dec.ExpectByte(')')
			return nil
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("expected SP between RETURN options")
		}
	}
}
