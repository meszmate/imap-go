package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Rename returns a handler for the RENAME command.
// RENAME changes the name of a mailbox.
func Rename() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		oldName, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing new mailbox name")
		}

		newName, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid new mailbox name")
		}

		if err := ctx.Session.Rename(oldName, newName); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "RENAME completed")
		return nil
	}
}
