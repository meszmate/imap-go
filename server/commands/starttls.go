package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// StartTLS returns a handler for the STARTTLS command.
// STARTTLS upgrades the connection to use TLS.
func StartTLS() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Conn.IsTLS() {
			return imap.ErrBad("already using TLS")
		}

		tlsConfig := ctx.Server.Options().TLSConfig
		if tlsConfig == nil {
			return imap.ErrNo("STARTTLS not available")
		}

		// Send OK before upgrading
		ctx.Conn.WriteOK(ctx.Tag, "Begin TLS negotiation now")

		// Upgrade the connection
		if err := ctx.Conn.UpgradeTLS(tlsConfig); err != nil {
			return imap.ErrNo("TLS negotiation failed")
		}

		return nil
	}
}
