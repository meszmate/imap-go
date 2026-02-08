package commands

import (
	"github.com/meszmate/imap-go/server"
)

// Capability returns a handler for the CAPABILITY command.
// CAPABILITY lists the capabilities supported by the server.
func Capability() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		ctx.Conn.WriteCapabilities()
		ctx.Conn.WriteOK(ctx.Tag, "CAPABILITY completed")
		return nil
	}
}
