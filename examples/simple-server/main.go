// Command simple-server demonstrates basic IMAP server usage with an in-memory backend.
package main

import (
	"log"
	"log/slog"
	"os"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/server/memserver"
)

func main() {
	addr := ":143"
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create in-memory backend with a test user
	mem := memserver.New()
	mem.AddUser("demo", "demo")

	// Add some sample messages to the INBOX
	userData := mem.GetUserData("demo")
	if userData != nil {
		inbox := userData.GetMailbox("INBOX")
		if inbox != nil {
			inbox.Append(
				[]byte("From: sender@example.com\r\nTo: demo@example.com\r\nSubject: Welcome\r\nDate: Mon, 1 Jan 2024 00:00:00 +0000\r\n\r\nWelcome to imap-go!\r\n"),
				[]imap.Flag{imap.FlagSeen},
				time.Now(),
			)
			inbox.Append(
				[]byte("From: test@example.com\r\nTo: demo@example.com\r\nSubject: Test Message\r\nDate: Mon, 1 Jan 2024 01:00:00 +0000\r\n\r\nThis is a test message.\r\n"),
				nil,
				time.Now(),
			)
		}
	}

	// Create server
	srv := server.New(
		server.WithLogger(logger),
		server.WithNewSession(func(conn *server.Conn) (server.Session, error) {
			return mem.NewSession(conn)
		}),
		server.WithGreetingText("imap-go demo server ready"),
	)

	log.Printf("Starting IMAP server on %s (user: demo, password: demo)", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
