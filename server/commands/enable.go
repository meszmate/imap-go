package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Enable returns a handler for the ENABLE command (RFC 5161).
// ENABLE allows the client to enable server extensions.
func Enable() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing capabilities")
		}

		var requested []imap.Cap
		for {
			cap, err := ctx.Decoder.ReadAtom()
			if err != nil {
				break
			}
			requested = append(requested, imap.Cap(strings.ToUpper(cap)))

			if err := ctx.Decoder.ReadSP(); err != nil {
				break
			}
		}

		if len(requested) == 0 {
			return imap.ErrBad("missing capabilities to enable")
		}

		// Check which capabilities can be enabled
		serverCaps := ctx.Server.Capabilities(ctx.Conn)
		serverCapSet := imap.NewCapSet(serverCaps...)

		var enabled []imap.Cap
		for _, cap := range requested {
			if serverCapSet.Has(cap) {
				ctx.Conn.Enabled().Add(cap)
				enabled = append(enabled, cap)
			}
		}

		// Write ENABLED response
		enc := ctx.Conn.Encoder()
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("ENABLED")
			for _, cap := range enabled {
				e.SP().Atom(string(cap))
			}
			e.CRLF()
		})

		ctx.Conn.WriteOK(ctx.Tag, "ENABLE completed")
		return nil
	}
}
