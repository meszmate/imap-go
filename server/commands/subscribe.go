package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Subscribe returns a handler for the SUBSCRIBE command.
// SUBSCRIBE adds the specified mailbox to the subscription list.
func Subscribe() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing mailbox name")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Session.Subscribe(mailbox); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "SUBSCRIBE completed")
		return nil
	}
}

// Unsubscribe returns a handler for the UNSUBSCRIBE command.
// UNSUBSCRIBE removes the specified mailbox from the subscription list.
func Unsubscribe() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing mailbox name")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Session.Unsubscribe(mailbox); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "UNSUBSCRIBE completed")
		return nil
	}
}
