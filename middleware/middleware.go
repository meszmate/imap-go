// Package middleware provides a middleware pipeline for the IMAP server.
//
// Middleware wraps command handlers to add cross-cutting concerns like
// logging, rate limiting, metrics, panic recovery, and timeouts.
package middleware

import (
	"github.com/meszmate/imap-go/server"
)

// Middleware wraps a CommandHandler to add behavior before/after handling.
type Middleware func(next server.CommandHandler) server.CommandHandler

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in order: the first middleware in the list
// is the outermost (executed first on request, last on response).
func Chain(middlewares ...Middleware) Middleware {
	return func(next server.CommandHandler) server.CommandHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Apply applies a middleware to all registered handlers in a server.
func Apply(srv *server.Server, mw Middleware) {
	for _, name := range srv.Dispatcher().Names() {
		srv.WrapHandler(name, func(h server.CommandHandler) server.CommandHandler {
			return mw(h)
		})
	}
}

// ApplyChain applies a chain of middlewares to all registered handlers.
func ApplyChain(srv *server.Server, middlewares ...Middleware) {
	mw := Chain(middlewares...)
	Apply(srv, mw)
}
