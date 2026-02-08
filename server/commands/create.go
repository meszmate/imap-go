package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Create returns a handler for the CREATE command.
// CREATE creates a new mailbox.
func Create() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing mailbox name")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Session.Create(mailbox, &imap.CreateOptions{}); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "CREATE completed")
		return nil
	}
}
