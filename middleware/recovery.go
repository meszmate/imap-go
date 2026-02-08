package middleware

import (
	"fmt"
	"runtime/debug"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Recovery returns a middleware that recovers from panics in command handlers.
func Recovery() Middleware {
	return func(next server.CommandHandler) server.CommandHandler {
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					ctx.Conn.Logger().Error("panic in command handler",
						"command", ctx.Name,
						"panic", fmt.Sprintf("%v", r),
						"stack", string(stack),
					)
					err = imap.ErrNo("internal server error")
				}
			}()

			return next.Handle(ctx)
		})
	}
}
