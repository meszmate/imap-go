package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Select returns a handler for the SELECT command.
// SELECT opens a mailbox in read-write mode.
func Select() server.CommandHandlerFunc {
	return handleSelect(false)
}

// Examine returns a handler for the EXAMINE command.
// EXAMINE opens a mailbox in read-only mode.
func Examine() server.CommandHandlerFunc {
	return handleSelect(true)
}

func handleSelect(readOnly bool) server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing mailbox name")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		options := &imap.SelectOptions{
			ReadOnly: readOnly,
		}

		data, err := ctx.Session.Select(mailbox, options)
		if err != nil {
			return err
		}

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
}
