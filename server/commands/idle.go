package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Idle returns a handler for the IDLE command (RFC 2177).
// IDLE allows the server to send unsolicited updates to the client.
func Idle() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		// Send continuation request
		enc := ctx.Conn.Encoder()
		enc.Encode(func(e *wire.Encoder) {
			e.ContinuationRequest("idling")
		})

		// Create a stop channel for idle
		stop := make(chan struct{})

		// Start a goroutine to wait for DONE from the client
		doneCh := make(chan error, 1)
		go func() {
			// Read lines from the connection until we get DONE
			connDec := ctx.Conn.Decoder()
			for {
				line, err := connDec.ReadLine()
				if err != nil {
					doneCh <- err
					return
				}
				if strings.EqualFold(strings.TrimSpace(line), "DONE") {
					close(stop)
					doneCh <- nil
					return
				}
			}
		}()

		// Call session.Idle which blocks until stop is closed
		w := server.NewUpdateWriter(ctx.Conn.Encoder())
		idleErr := ctx.Session.Idle(w, stop)

		// Wait for the DONE reader to finish
		readErr := <-doneCh

		if idleErr != nil {
			return idleErr
		}
		if readErr != nil {
			return imap.ErrBad("IDLE terminated: " + readErr.Error())
		}

		ctx.Conn.WriteOK(ctx.Tag, "IDLE completed")
		return nil
	}
}
