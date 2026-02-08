package commands

import (
	"github.com/meszmate/imap-go/server"
)

// Noop returns a handler for the NOOP command.
// NOOP does nothing but elicit a tagged OK response.
func Noop() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		ctx.Conn.WriteOK(ctx.Tag, "NOOP completed")
		return nil
	}
}
