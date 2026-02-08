// Command proxy demonstrates an IMAP proxy that forwards commands to an upstream server
// with middleware applied.
package main

import (
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/meszmate/imap-go/middleware"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/server/memserver"
)

func main() {
	listenAddr := ":10143"
	if len(os.Args) >= 2 {
		listenAddr = os.Args[1]
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	_ = logger // logger available for custom middleware

	// Create backend (in production, this would proxy to an upstream IMAP server)
	mem := memserver.New()
	mem.AddUser("user", "pass")

	// Create server with middleware
	srv := server.New(
		server.WithNewSession(func(conn *server.Conn) (server.Session, error) {
			return mem.NewSession(conn)
		}),
	)

	// Build middleware chain
	chain := middleware.Chain(
		middleware.Recovery(),
		middleware.Logging(),
		middleware.Timeout(30*time.Second),
		middleware.RateLimit(middleware.RateLimitConfig{
			MaxCommandsPerSecond: 100,
			BurstSize:            10,
		}),
	)

	// Apply middleware to all registered handlers
	for _, name := range srv.Dispatcher().Names() {
		srv.WrapHandler(name, func(next server.CommandHandler) server.CommandHandler {
			return chain(next)
		})
	}

	log.Printf("Starting IMAP proxy on %s", listenAddr)
	if err := srv.ListenAndServe(listenAddr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
