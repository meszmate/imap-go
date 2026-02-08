package middleware

import (
	"context"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Timeout returns a middleware that enforces a timeout on command execution.
func Timeout(d time.Duration) Middleware {
	return func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			timeoutCtx, cancel := context.WithTimeout(ctx.Context, d)
			defer cancel()

			ctx.Context = timeoutCtx

			done := make(chan error, 1)
			go func() {
				done <- next.Handle(ctx)
			}()

			select {
			case err := <-done:
				return err
			case <-timeoutCtx.Done():
				return imap.ErrNo("command timed out")
			}
		})
	}
}
