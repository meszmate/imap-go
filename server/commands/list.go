package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// List returns a handler for the LIST command.
// LIST returns a subset of mailbox names from the complete set of all names
// available to the client.
func List() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		// Read reference name
		ref, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid reference name")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing mailbox pattern")
		}

		// Read mailbox pattern
		pattern, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox pattern")
		}

		patterns := []string{pattern}
		options := &imap.ListOptions{}

		w := server.NewListWriter(ctx.Conn.Encoder())
		if err := ctx.Session.List(w, ref, patterns, options); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "LIST completed")
		return nil
	}
}

// Lsub returns a handler for the LSUB command.
// LSUB returns a subset of subscribed mailbox names.
// This is implemented as LIST with the SelectSubscribed option.
func Lsub() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		// Read reference name
		ref, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid reference name")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing mailbox pattern")
		}

		// Read mailbox pattern
		pattern, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox pattern")
		}

		patterns := []string{pattern}
		options := &imap.ListOptions{
			SelectSubscribed: true,
		}

		w := server.NewListWriter(ctx.Conn.Encoder())
		if err := ctx.Session.List(w, ref, patterns, options); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "LSUB completed")
		return nil
	}
}
