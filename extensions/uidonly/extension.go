// Package uidonly implements the UIDONLY extension (RFC 9586).
//
// UIDONLY enables a UID-only mode where the server stops using message
// sequence numbers in untagged responses and instead uses UIDs exclusively.
// It is activated via the ENABLE command (ENABLE UIDONLY), after which:
//   - Sequence-number-based commands are rejected with BAD [UIDREQUIRED]
//   - FETCH responses become UIDFETCH using UIDs instead of sequence numbers
//   - EXPUNGE responses become VANISHED with UID sets
//
// This extension wraps ENABLE, FETCH, STORE, COPY, SEARCH, EXPUNGE, MOVE,
// SORT, THREAD, SELECT, and EXAMINE to enforce UID-only mode.
package uidonly

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/condstore"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionUIDOnly is the session interface for UIDONLY support.
// Backends implement this to be notified when a client enables UID-only
// mode, allowing the session to adjust its response format to use UIDs
// exclusively instead of message sequence numbers.
type SessionUIDOnly interface {
	// EnableUIDOnly is called when a client enables UID-only mode via
	// ENABLE UIDONLY. After this call, the session should use UIDs
	// instead of sequence numbers in all untagged responses including
	// FETCH, STORE, SEARCH, and EXPUNGE.
	EnableUIDOnly() error
}

// Extension implements the UIDONLY extension (RFC 9586).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UIDONLY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UIDONLY",
			ExtCapabilities: []imap.Cap{imap.CapUIDOnly},
			ExtDependencies: []string{"CONDSTORE"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// UIDONLY is activated via the ENABLE command, which is handled
// separately, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps existing command handlers to enforce UIDONLY mode.
// It rejects sequence-number-based commands and rewrites responses to use
// UIDFETCH and VANISHED formats when UIDONLY is enabled.
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
	case "ENABLE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlyEnable(ctx, h)
		})
	case "FETCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlyFetch(ctx, h)
		})
	case "STORE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlyStore(ctx, h)
		})
	case "COPY":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySeqReject(ctx, h)
		})
	case "SEARCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySeqReject(ctx, h)
		})
	case "EXPUNGE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlyExpunge(ctx, h)
		})
	case "MOVE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlyMove(ctx, h)
		})
	case "SORT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySeqReject(ctx, h)
		})
	case "THREAD":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySeqReject(ctx, h)
		})
	case "SELECT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySelect(ctx, h)
		})
	case "EXAMINE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUIDOnlySelect(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns a typed nil pointer to SessionUIDOnly, indicating
// that sessions should implement this interface for full UIDONLY support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUIDOnly)(nil)
}

// OnEnabled is called when a client enables UIDONLY via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// uidOnlyEnabled returns true if UIDONLY is enabled on the connection.
func uidOnlyEnabled(ctx *server.CommandContext) bool {
	return ctx.Conn.Enabled().Has(imap.CapUIDOnly)
}

// errUIDRequired returns a BAD [UIDREQUIRED] error for seq-mode commands.
func errUIDRequired() *imap.IMAPError {
	return imap.ErrBadWithCode(imap.ResponseCodeUIDRequired,
		"message sequence numbers are not allowed when UIDONLY is enabled")
}

// handleUIDOnlyEnable wraps ENABLE to detect UIDONLY activation and notify the session.
func handleUIDOnlyEnable(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if err := original(ctx); err != nil {
		return err
	}

	if !ctx.Conn.Enabled().Has(imap.CapUIDOnly) {
		return nil
	}

	if sess, ok := ctx.Session.(SessionUIDOnly); ok {
		return sess.EnableUIDOnly()
	}

	return nil
}

// handleUIDOnlySeqReject rejects seq-mode commands when UIDONLY is enabled,
// otherwise delegates to the original handler. Used for COPY, SEARCH, SORT, THREAD.
func handleUIDOnlySeqReject(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if uidOnlyEnabled(ctx) && ctx.NumKind == server.NumKindSeq {
		return errUIDRequired()
	}
	return original(ctx)
}

// handleUIDOnlyFetch handles FETCH with UIDONLY support.
// When UIDONLY is enabled:
//   - Seq-mode FETCH is rejected with UIDREQUIRED
//   - UID FETCH creates a UIDONLY-aware FetchWriter that outputs UIDFETCH format
func handleUIDOnlyFetch(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !uidOnlyEnabled(ctx) {
		return original(ctx)
	}

	if ctx.NumKind == server.NumKindSeq {
		return errUIDRequired()
	}

	// UID FETCH with UIDONLY enabled — handle ourselves with UIDFETCH writer
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	// Read UID set
	uidSetStr, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid UID set")
	}
	uidSet, err := imap.ParseUIDSet(uidSetStr)
	if err != nil {
		return imap.ErrBad("invalid UID set")
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing fetch items")
	}

	// Parse fetch items using CONDSTORE's exported parser
	options, err := condstore.ParseFetchItems(dec)
	if err != nil {
		return imap.ErrBad("invalid fetch items: " + err.Error())
	}

	// Parse optional (CHANGEDSINCE <modseq> [VANISHED]) modifier
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
						if ctx.Conn.Enabled().Has(imap.CapQResync) {
							options.Vanished = true
						}
					}
				}
			}

			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for modifier")
			}
		}
	}

	// Always include UID for UID FETCH
	options.UID = true

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	w.SetUIDOnly(true)
	if err := ctx.Session.Fetch(w, uidSet, options); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "FETCH completed")
	return nil
}

// handleUIDOnlyStore handles STORE with UIDONLY support.
// When UIDONLY is enabled:
//   - Seq-mode STORE is rejected with UIDREQUIRED
//   - UID STORE creates a UIDONLY-aware FetchWriter that outputs UIDFETCH format
func handleUIDOnlyStore(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !uidOnlyEnabled(ctx) {
		return original(ctx)
	}

	if ctx.NumKind == server.NumKindSeq {
		return errUIDRequired()
	}

	// UID STORE with UIDONLY enabled — handle ourselves with UIDFETCH writer
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	// Read UID set
	uidSetStr, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid UID set")
	}
	uidSet, err := imap.ParseUIDSet(uidSetStr)
	if err != nil {
		return imap.ErrBad("invalid UID set")
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing store action")
	}

	// Check for optional (UNCHANGEDSINCE <modseq>) modifier
	storeOptions := &imap.StoreOptions{}
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end of command")
	}

	if b == '(' {
		if err := dec.ExpectByte('('); err != nil {
			return imap.ErrBad("invalid modifier")
		}
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid modifier name")
		}
		if !strings.EqualFold(atom, "UNCHANGEDSINCE") {
			return imap.ErrBad("unknown store modifier: " + atom)
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing UNCHANGEDSINCE value")
		}
		modseq, err := dec.ReadNumber64()
		if err != nil {
			return imap.ErrBad("invalid UNCHANGEDSINCE value")
		}
		if err := dec.ExpectByte(')'); err != nil {
			return imap.ErrBad("missing closing paren for modifier")
		}
		storeOptions.UnchangedSince = modseq

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing store action")
		}
	}

	// Read store action
	actionStr, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid store action")
	}

	storeFlags := &imap.StoreFlags{}
	upper := strings.ToUpper(actionStr)

	switch {
	case strings.HasPrefix(upper, "+FLAGS"):
		storeFlags.Action = imap.StoreFlagsAdd
	case strings.HasPrefix(upper, "-FLAGS"):
		storeFlags.Action = imap.StoreFlagsDel
	case strings.HasPrefix(upper, "FLAGS"):
		storeFlags.Action = imap.StoreFlagsSet
	default:
		return imap.ErrBad("invalid store action: " + actionStr)
	}

	if strings.HasSuffix(upper, ".SILENT") {
		storeFlags.Silent = true
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing flags")
	}

	flagStrs, err := dec.ReadFlags()
	if err != nil {
		return imap.ErrBad("invalid flags")
	}

	for _, f := range flagStrs {
		storeFlags.Flags = append(storeFlags.Flags, imap.Flag(f))
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	w.SetUIDOnly(true)

	if storeOptions.UnchangedSince > 0 {
		if sess, ok := ctx.Session.(condstore.SessionCondStore); ok {
			if err := sess.StoreConditional(w, uidSet, storeFlags, storeOptions); err != nil {
				return err
			}
		} else {
			if err := ctx.Session.Store(w, uidSet, storeFlags, storeOptions); err != nil {
				return err
			}
		}
	} else {
		if err := ctx.Session.Store(w, uidSet, storeFlags, storeOptions); err != nil {
			return err
		}
	}

	ctx.Conn.WriteOK(ctx.Tag, "STORE completed")
	return nil
}

// handleUIDOnlyExpunge handles EXPUNGE with UIDONLY support.
// When UIDONLY is enabled, creates a VANISHED-aware ExpungeWriter.
func handleUIDOnlyExpunge(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !uidOnlyEnabled(ctx) {
		return original(ctx)
	}

	// EXPUNGE with UIDONLY — create VANISHED-emitting writer
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
	w.SetUIDOnly(true)
	if err := ctx.Session.Expunge(w, uids); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "EXPUNGE completed")
	return nil
}

// handleUIDOnlyMove handles MOVE with UIDONLY support.
// When UIDONLY is enabled:
//   - Seq-mode MOVE is rejected with UIDREQUIRED
//   - UID MOVE creates a VANISHED-aware MoveWriter
func handleUIDOnlyMove(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !uidOnlyEnabled(ctx) {
		return original(ctx)
	}

	if ctx.NumKind == server.NumKindSeq {
		return errUIDRequired()
	}

	// UID MOVE with UIDONLY — create VANISHED-emitting writer
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	setStr, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid message set")
	}

	uidSet, err := imap.ParseUIDSet(setStr)
	if err != nil {
		return imap.ErrBad("invalid UID set")
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing destination mailbox")
	}

	dest, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid destination mailbox")
	}

	sessMove, ok := ctx.Session.(server.SessionMove)
	if !ok {
		return imap.ErrNo("MOVE not supported")
	}

	w := server.NewMoveWriter(ctx.Conn.Encoder())
	w.SetUIDOnly(true)
	if err := sessMove.Move(w, uidSet, dest); err != nil {
		return err
	}

	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(ctx.Tag, "OK", "", "MOVE completed")
	})

	return nil
}

// handleUIDOnlySelect wraps SELECT/EXAMINE to reject QRESYNC SeqMatch
// when UIDONLY is enabled. RFC 9586 forbids sequence-to-UID mappings
// in UIDONLY mode since sequence numbers are not meaningful.
func handleUIDOnlySelect(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !uidOnlyEnabled(ctx) || ctx.Decoder == nil {
		return original(ctx)
	}

	// Read all remaining bytes from the decoder so we can scan for QRESYNC
	// SeqMatch without losing data. Then re-create the decoder for the
	// original handler.
	var buf strings.Builder
	for {
		b, err := ctx.Decoder.PeekByte()
		if err != nil {
			break
		}
		_ = ctx.Decoder.ExpectByte(b)
		buf.WriteByte(b)
	}
	raw := buf.String()

	if hasQResyncSeqMatch(raw) {
		return imap.ErrBadWithCode(imap.ResponseCodeUIDRequired,
			"QRESYNC sequence-to-UID mapping not allowed when UIDONLY is enabled")
	}

	// Re-create the decoder with the original content and delegate.
	ctx.Decoder = wire.NewDecoder(strings.NewReader(raw))
	return original(ctx)
}

// hasQResyncSeqMatch checks if the raw SELECT/EXAMINE args contain a
// QRESYNC parameter with a SeqMatch (nested parenthesized pair).
// Format: ... (QRESYNC (<uidval> <modseq> [<known-uids> [(<seq-set> <uid-set>)]]))
// A nested '(' inside the QRESYNC params indicates SeqMatch.
func hasQResyncSeqMatch(raw string) bool {
	upper := strings.ToUpper(raw)
	idx := strings.Index(upper, "QRESYNC")
	if idx < 0 {
		return false
	}
	rest := raw[idx+len("QRESYNC"):]
	// Find the opening paren of QRESYNC params
	parenStart := strings.IndexByte(rest, '(')
	if parenStart < 0 {
		return false
	}
	inner := rest[parenStart+1:]
	// If we find a '(' inside the QRESYNC params, that's the SeqMatch pair.
	// The uidval, modseq, and known-uids fields are atoms without parens.
	for i := 0; i < len(inner); i++ {
		if inner[i] == '(' {
			return true
		}
		if inner[i] == ')' {
			return false
		}
	}
	return false
}

