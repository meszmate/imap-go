// Package qresync implements the QRESYNC extension (RFC 7162).
//
// QRESYNC (Quick Resynchronization) allows a client to efficiently
// resynchronize its local cache with the server by providing known UIDs
// and modification sequences during SELECT. It depends on CONDSTORE.
//
// The core SelectOptions already supports the QResync field with all necessary
// sub-fields (UIDValidity, ModSeq, KnownUIDs, SeqMatch). This extension
// parses QRESYNC parameters in SELECT/EXAMINE and VANISHED in FETCH,
// and advertises the QRESYNC capability.
package qresync

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/condstore"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionQResync is the session interface for QRESYNC support.
// Backends implement this to handle SELECT with QRESYNC parameters,
// which allows efficient mailbox resynchronization by providing known
// UIDs, modification sequences, and optional sequence-to-UID mappings.
type SessionQResync interface {
	// SelectQResync opens a mailbox with QRESYNC parameters.
	// The options.QResync field contains UIDValidity, ModSeq, KnownUIDs,
	// and optional SeqMatch data for efficient resynchronization.
	SelectQResync(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error)
}

// Extension implements the QRESYNC extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new QRESYNC extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "QRESYNC",
			ExtCapabilities: []imap.Cap{imap.CapQResync},
			ExtDependencies: []string{"CONDSTORE"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// QRESYNC modifies the SELECT command with additional parameters rather than
// adding new commands, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps existing command handlers to add QRESYNC parsing.
// It wraps SELECT/EXAMINE (QRESYNC parameters) and FETCH (VANISHED modifier).
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	switch name {
	case "SELECT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleQResyncSelect(ctx, false)
		})
	case "EXAMINE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleQResyncSelect(ctx, true)
		})
	case "FETCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleQResyncFetch(ctx)
		})
	}
	return nil
}

// SessionExtension returns a typed nil pointer to SessionQResync, indicating
// that sessions should implement this interface for full QRESYNC support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionQResync)(nil)
}

// OnEnabled is called when a client enables QRESYNC via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleQResyncSelect wraps SELECT/EXAMINE to parse both CONDSTORE and QRESYNC parameters.
//
// Format: SELECT <mailbox> (CONDSTORE)
// Format: SELECT <mailbox> (QRESYNC (<uidvalidity> <modseq> [<known-uids> [(<seq-set> <uid-set>)]]))
func handleQResyncSelect(ctx *server.CommandContext, readOnly bool) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing mailbox name")
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid mailbox name")
	}

	options := &imap.SelectOptions{
		ReadOnly: readOnly,
	}

	// Check for parameters after mailbox name
	if err := dec.ReadSP(); err == nil {
		b, err := dec.PeekByte()
		if err == nil && b == '(' {
			if err := dec.ExpectByte('('); err != nil {
				return imap.ErrBad("invalid SELECT parameters")
			}

			// Read parameters inside the outer list
			for {
				b, err := dec.PeekByte()
				if err != nil {
					return imap.ErrBad("unexpected end of SELECT parameters")
				}
				if b == ')' {
					if err := dec.ExpectByte(')'); err != nil {
						return imap.ErrBad("invalid SELECT parameters")
					}
					break
				}

				atom, err := dec.ReadAtom()
				if err != nil {
					return imap.ErrBad("invalid SELECT parameter")
				}

				switch strings.ToUpper(atom) {
				case "CONDSTORE":
					options.CondStore = true
				case "QRESYNC":
					qr, err := parseQResyncParams(dec)
					if err != nil {
						return err
					}
					options.QResync = qr
				default:
					return imap.ErrBad("unknown SELECT parameter: " + atom)
				}

				// Check for SP before next parameter or closing paren
				b, err = dec.PeekByte()
				if err != nil {
					return imap.ErrBad("unexpected end of SELECT parameters")
				}
				if b == ' ' {
					if err := dec.ReadSP(); err != nil {
						return imap.ErrBad("invalid SELECT parameters")
					}
				}
			}
		}
	}

	// If QRESYNC is requested, validate and handle
	if options.QResync != nil {
		if !ctx.Conn.Enabled().Has(imap.CapQResync) {
			return imap.ErrBad("QRESYNC not enabled; use ENABLE QRESYNC first")
		}
		// QRESYNC implies CONDSTORE
		options.CondStore = true

		sess, ok := ctx.Session.(SessionQResync)
		if ok {
			data, err := sess.SelectQResync(mailbox, options)
			if err != nil {
				return err
			}
			return writeSelectResponse(ctx, mailbox, data)
		}
		// Fall back to plain Select if session doesn't implement QRESYNC
	}

	data, err := ctx.Session.Select(mailbox, options)
	if err != nil {
		return err
	}

	return writeSelectResponse(ctx, mailbox, data)
}

// parseQResyncParams parses QRESYNC parameters: SP (uidvalidity SP modseq [SP known-uids [SP (seq-set SP uid-set)]])
func parseQResyncParams(dec *wire.Decoder) (*imap.SelectQResync, error) {
	if err := dec.ReadSP(); err != nil {
		return nil, imap.ErrBad("missing QRESYNC parameters")
	}
	if err := dec.ExpectByte('('); err != nil {
		return nil, imap.ErrBad("expected '(' for QRESYNC parameters")
	}

	qr := &imap.SelectQResync{}

	// Read uidvalidity (required)
	uidvalidity, err := dec.ReadNumber()
	if err != nil {
		return nil, imap.ErrBad("invalid QRESYNC uidvalidity")
	}
	qr.UIDValidity = uidvalidity

	if err := dec.ReadSP(); err != nil {
		return nil, imap.ErrBad("missing QRESYNC modseq")
	}

	// Read modseq (required)
	modseq, err := dec.ReadNumber64()
	if err != nil {
		return nil, imap.ErrBad("invalid QRESYNC modseq")
	}
	qr.ModSeq = modseq

	// Check for optional known-uids
	b, err := dec.PeekByte()
	if err != nil {
		return nil, imap.ErrBad("unexpected end of QRESYNC parameters")
	}
	if b == ')' {
		if err := dec.ExpectByte(')'); err != nil {
			return nil, imap.ErrBad("invalid QRESYNC parameters")
		}
		return qr, nil
	}

	if err := dec.ReadSP(); err != nil {
		return nil, imap.ErrBad("invalid QRESYNC parameters")
	}

	// Check if next is '(' (seq-match without known-uids) or atom (known-uids)
	b, err = dec.PeekByte()
	if err != nil {
		return nil, imap.ErrBad("unexpected end of QRESYNC parameters")
	}

	if b != '(' {
		// Read known-uids
		knownUIDsStr, err := dec.ReadAtom()
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC known-uids")
		}
		knownUIDs, err := imap.ParseUIDSet(knownUIDsStr)
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC known-uids: " + err.Error())
		}
		qr.KnownUIDs = knownUIDs

		// Check for optional seq-match
		b, err = dec.PeekByte()
		if err != nil {
			return nil, imap.ErrBad("unexpected end of QRESYNC parameters")
		}
		if b == ')' {
			if err := dec.ExpectByte(')'); err != nil {
				return nil, imap.ErrBad("invalid QRESYNC parameters")
			}
			return qr, nil
		}

		if err := dec.ReadSP(); err != nil {
			return nil, imap.ErrBad("invalid QRESYNC parameters")
		}
	}

	// Parse optional seq-match: (seq-set SP uid-set)
	b, err = dec.PeekByte()
	if err != nil {
		return nil, imap.ErrBad("unexpected end of QRESYNC parameters")
	}
	if b == '(' {
		if err := dec.ExpectByte('('); err != nil {
			return nil, imap.ErrBad("invalid QRESYNC seq-match")
		}

		seqSetStr, err := dec.ReadAtom()
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC seq-match seq-set")
		}
		seqSet, err := imap.ParseSeqSet(seqSetStr)
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC seq-match seq-set: " + err.Error())
		}

		if err := dec.ReadSP(); err != nil {
			return nil, imap.ErrBad("missing QRESYNC seq-match uid-set")
		}

		uidSetStr, err := dec.ReadAtom()
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC seq-match uid-set")
		}
		uidSet, err := imap.ParseUIDSet(uidSetStr)
		if err != nil {
			return nil, imap.ErrBad("invalid QRESYNC seq-match uid-set: " + err.Error())
		}

		if err := dec.ExpectByte(')'); err != nil {
			return nil, imap.ErrBad("missing closing paren for QRESYNC seq-match")
		}

		qr.SeqMatch = &imap.QResyncSeqMatch{
			SeqNums: seqSet,
			UIDs:    uidSet,
		}
	}

	// Close QRESYNC params
	if err := dec.ExpectByte(')'); err != nil {
		return nil, imap.ErrBad("missing closing paren for QRESYNC parameters")
	}

	return qr, nil
}

// writeSelectResponse writes the standard SELECT/EXAMINE response.
func writeSelectResponse(ctx *server.CommandContext, mailbox string, data *imap.SelectData) error {
	enc := ctx.Conn.Encoder()

	// Write FLAGS
	flagStrs := make([]string, len(data.Flags))
	for i, f := range data.Flags {
		flagStrs[i] = string(f)
	}
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("FLAGS").SP().Flags(flagStrs).CRLF()
	})

	// Write EXISTS
	enc.Encode(func(e *wire.Encoder) {
		e.NumResponse(data.NumMessages, "EXISTS")
	})

	// Write RECENT
	enc.Encode(func(e *wire.Encoder) {
		e.NumResponse(data.NumRecent, "RECENT")
	})

	// Write UIDVALIDITY
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("OK").SP()
		e.ResponseCode("UIDVALIDITY", data.UIDValidity)
		e.CRLF()
	})

	// Write UIDNEXT
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("OK").SP()
		e.ResponseCode("UIDNEXT", uint32(data.UIDNext))
		e.CRLF()
	})

	// Write PERMANENTFLAGS if present
	if len(data.PermanentFlags) > 0 {
		permFlagStrs := make([]string, len(data.PermanentFlags))
		for i, f := range data.PermanentFlags {
			permFlagStrs[i] = string(f)
		}
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.RawString("[PERMANENTFLAGS ")
			e.Flags(permFlagStrs)
			e.RawString("] ")
			e.CRLF()
		})
	}

	// Write UNSEEN if present
	if data.FirstUnseen > 0 {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.ResponseCode("UNSEEN", data.FirstUnseen)
			e.CRLF()
		})
	}

	// Write HIGHESTMODSEQ if present
	if data.HighestModSeq > 0 {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.ResponseCode("HIGHESTMODSEQ", data.HighestModSeq)
			e.CRLF()
		})
	}

	// Write VANISHED (EARLIER) if present (QRESYNC)
	if data.Vanished != nil && !data.Vanished.IsEmpty() {
		vanished := data.Vanished.String()
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("VANISHED").SP().Atom("(EARLIER)").SP().Atom(vanished).CRLF()
		})
	}

	// Update connection state
	ctx.Conn.SetMailbox(mailbox, data.ReadOnly)
	if err := ctx.Conn.SetState(imap.ConnStateSelected); err != nil {
		return err
	}

	// Tagged OK with READ-ONLY or READ-WRITE code
	code := "READ-WRITE"
	if data.ReadOnly {
		code = "READ-ONLY"
	}
	enc.Encode(func(e *wire.Encoder) {
		e.StatusResponse(ctx.Tag, "OK", code, "SELECT completed")
	})

	return nil
}

// handleQResyncFetch wraps the FETCH command to parse (CHANGEDSINCE <modseq> VANISHED).
//
// Format: UID FETCH <seqset> <items> (CHANGEDSINCE <modseq> VANISHED)
func handleQResyncFetch(ctx *server.CommandContext) error {
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

	// Parse fetch items
	options, err := condstore.ParseFetchItems(dec)
	if err != nil {
		return imap.ErrBad("invalid fetch items: " + err.Error())
	}

	// Check for (CHANGEDSINCE <modseq> [VANISHED]) modifier after fetch items
	if err := dec.ReadSP(); err == nil {
		b, err := dec.PeekByte()
		if err == nil && b == '(' {
			if err := dec.ExpectByte('('); err != nil {
				return imap.ErrBad("invalid modifier")
			}
			atom, err := dec.ReadAtom()
			if err != nil {
				return imap.ErrBad("invalid modifier name")
			}
			if !strings.EqualFold(atom, "CHANGEDSINCE") {
				return imap.ErrBad("unknown fetch modifier: " + atom)
			}
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing CHANGEDSINCE value")
			}
			modseq, err := dec.ReadNumber64()
			if err != nil {
				return imap.ErrBad("invalid CHANGEDSINCE value")
			}
			options.ChangedSince = modseq
			options.ModSeq = true

			// Check for optional VANISHED keyword
			b, err = dec.PeekByte()
			if err == nil && b == ' ' {
				if err := dec.ReadSP(); err == nil {
					vanAtom, err := dec.ReadAtom()
					if err == nil && strings.EqualFold(vanAtom, "VANISHED") {
						if !ctx.Conn.Enabled().Has(imap.CapQResync) {
							return imap.ErrBad("QRESYNC not enabled; use ENABLE QRESYNC first")
						}
						if ctx.NumKind != server.NumKindUID {
							return imap.ErrBad("VANISHED requires UID FETCH")
						}
						options.Vanished = true
					}
				}
			}

			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for modifier")
			}
		}
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
