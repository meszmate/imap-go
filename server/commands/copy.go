package commands

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Copy returns a handler for the COPY command.
// COPY copies the specified messages to the end of the specified destination mailbox.
func Copy() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
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

		data, err := ctx.Session.Copy(numSet, dest)
		if err != nil {
			return err
		}

		// Write tagged OK, optionally with COPYUID response code
		if data != nil && data.UIDValidity > 0 {
			enc := ctx.Conn.Encoder()
			enc.Encode(func(e *wire.Encoder) {
				code := fmt.Sprintf("COPYUID %d %s %s",
					data.UIDValidity,
					data.SourceUIDs.String(),
					data.DestUIDs.String())
				e.StatusResponse(ctx.Tag, "OK", code, "COPY completed")
			})
		} else {
			ctx.Conn.WriteOK(ctx.Tag, "COPY completed")
		}

		return nil
	}
}
