package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Delete returns a handler for the DELETE command.
// DELETE removes a mailbox.
func Delete() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing mailbox name")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Session.Delete(mailbox); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "DELETE completed")
		return nil
	}
}
