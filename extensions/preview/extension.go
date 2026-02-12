// Package preview implements the PREVIEW extension (RFC 8970).
//
// PREVIEW provides a short text preview of a message's content as a FETCH
// data item. It supports the PREVIEW (LAZY) modifier which allows the server
// to return NIL if the preview isn't pre-computed.
package preview

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/condstore"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionPreview is an optional interface for sessions that support
// fetching message previews (RFC 8970).
type SessionPreview interface {
	// FetchPreview returns a short text preview for the message with the given UID.
	// If lazy is true, the server MAY return nil if the preview is not pre-computed.
	FetchPreview(uid imap.UID, lazy bool) (*string, error)
}

// Extension implements the PREVIEW IMAP extension (RFC 8970).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new PREVIEW extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "PREVIEW",
			ExtCapabilities: []imap.Cap{imap.CapPreview},
			ExtDependencies: []string{"CONDSTORE"},
		},
	}
}

// CommandHandlers returns nil because PREVIEW modifies FETCH rather than
// adding new commands.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps the FETCH command handler to support PREVIEW (LAZY) parsing.
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
	case "FETCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handlePreviewFetch(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the SessionPreview interface that sessions may
// implement to provide message previews through a dedicated method.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionPreview)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handlePreviewFetch wraps the FETCH command to parse PREVIEW (LAZY) modifiers.
//
// Formats:
//
//	FETCH 1 (FLAGS PREVIEW)
//	FETCH 1 (FLAGS PREVIEW (LAZY))
//	FETCH 1 PREVIEW
//	FETCH 1 PREVIEW (LAZY)
//	FETCH 1 (FLAGS PREVIEW (LAZY)) (CHANGEDSINCE 123)
//	FETCH 1 PREVIEW (LAZY) (CHANGEDSINCE 123)
func handlePreviewFetch(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	// Read sequence set
	seqSetStr, err := dec.ReadAtom()
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

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing fetch items")
	}

	// Parse fetch items with PREVIEW (LAZY) support
	options, err := parsePreviewFetchItems(dec)
	if err != nil {
		return imap.ErrBad("invalid fetch items: " + err.Error())
	}

	// Check for post-item modifiers: (LAZY) and/or (CHANGEDSINCE <modseq>)
	if err := parsePostItemModifiers(dec, options); err != nil {
		return err
	}

	// Always include UID for UID FETCH
	if ctx.NumKind == server.NumKindUID {
		options.UID = true
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	if err := ctx.Session.Fetch(w, numSet, options); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "FETCH completed")
	return nil
}

// parsePreviewFetchItems parses FETCH item specifications with support for
// PREVIEW (LAZY) inside parenthesized lists.
//
// The standard condstore.ParseFetchItems uses dec.ReadList() which treats
// the ( in PREVIEW (LAZY) as a nested list start. We reimplement the list
// loop to handle this case.
func parsePreviewFetchItems(dec *wire.Decoder) (*imap.FetchOptions, error) {
	options := &imap.FetchOptions{}

	b, err := dec.PeekByte()
	if err != nil {
		return nil, err
	}

	if b == '(' {
		// Parenthesized list of items
		if err := dec.ExpectByte('('); err != nil {
			return nil, err
		}

		first := true
		for {
			// Check for closing paren
			b, err := dec.PeekByte()
			if err != nil {
				return nil, err
			}
			if b == ')' {
				if err := dec.ExpectByte(')'); err != nil {
					return nil, err
				}
				break
			}

			if !first {
				if err := dec.ReadSP(); err != nil {
					return nil, err
				}

				// After SP, check if this is a (LAZY) modifier for a preceding PREVIEW
				b, err := dec.PeekByte()
				if err != nil {
					return nil, err
				}
				if b == '(' && options.Preview {
					if err := parsePreviewModifier(dec, options); err != nil {
						return nil, err
					}

					// Check what comes next: ) to end list, or SP for more items
					continue
				}
			}
			first = false

			if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
				return nil, err
			}
		}
	} else {
		// Single item
		if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
			return nil, err
		}
	}

	return options, nil
}

// parsePreviewModifier parses the (LAZY) modifier after PREVIEW.
// Grammar: "(" preview-mod *(SP preview-mod) ")"
func parsePreviewModifier(dec *wire.Decoder, options *imap.FetchOptions) error {
	if err := dec.ExpectByte('('); err != nil {
		return err
	}

	first := true
	for {
		b, err := dec.PeekByte()
		if err != nil {
			return err
		}
		if b == ')' {
			if err := dec.ExpectByte(')'); err != nil {
				return err
			}
			return nil
		}

		if !first {
			if err := dec.ReadSP(); err != nil {
				return err
			}
		}
		first = false

		atom, err := dec.ReadAtom()
		if err != nil {
			return err
		}
		if strings.EqualFold(atom, "LAZY") {
			options.PreviewLazy = true
		} else {
			return imap.ErrBad("unknown preview modifier: " + atom)
		}
	}
}

// parsePostItemModifiers parses modifiers that appear after the fetch item list.
// These include (LAZY) for single-item PREVIEW and (CHANGEDSINCE <modseq>).
//
// Examples:
//
//	FETCH 1 PREVIEW (LAZY)
//	FETCH 1 PREVIEW (LAZY) (CHANGEDSINCE 123)
//	FETCH 1 (FLAGS PREVIEW) (CHANGEDSINCE 456)
func parsePostItemModifiers(dec *wire.Decoder, options *imap.FetchOptions) error {
	for {
		if err := dec.ReadSP(); err != nil {
			// No more modifiers
			return nil
		}

		b, err := dec.PeekByte()
		if err != nil {
			return nil
		}
		if b != '(' {
			return imap.ErrBad("expected modifier")
		}

		if err := dec.ExpectByte('('); err != nil {
			return imap.ErrBad("invalid modifier")
		}

		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid modifier name")
		}

		switch {
		case strings.EqualFold(atom, "LAZY") && options.Preview:
			options.PreviewLazy = true
			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for LAZY modifier")
			}

		case strings.EqualFold(atom, "CHANGEDSINCE"):
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing CHANGEDSINCE value")
			}
			modseq, err := dec.ReadNumber64()
			if err != nil {
				return imap.ErrBad("invalid CHANGEDSINCE value")
			}
			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for modifier")
			}
			options.ChangedSince = modseq
			options.ModSeq = true

		default:
			return imap.ErrBad("unknown fetch modifier: " + atom)
		}
	}
}
