package middleware

import (
	"time"

	"github.com/meszmate/imap-go/server"
)

// Logging returns a middleware that logs command execution.
func Logging() Middleware {
	return func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			start := time.Now()
			logger := ctx.Conn.Logger()

			logger.Info("command start",
				"tag", ctx.Tag,
				"command", ctx.Name,
				"state", ctx.State().String(),
			)

			err := next.Handle(ctx)
			duration := time.Since(start)

			if err != nil {
				logger.Warn("command error",
					"tag", ctx.Tag,
					"command", ctx.Name,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info("command done",
					"tag", ctx.Tag,
					"command", ctx.Name,
					"duration", duration,
				)
			}

			return err
		})
	}
}
