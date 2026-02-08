// Package imaptest provides test infrastructure for IMAP client/server testing.
package imaptest

import (
	"net"
	"testing"

	"github.com/meszmate/imap-go/client"
	"github.com/meszmate/imap-go/server"
)

// Harness provides an in-process IMAP server and client for testing.
type Harness struct {
	t      *testing.T
	server *server.Server
	listener net.Listener
	done   chan struct{}
}

// NewHarness creates a new test harness with the given server.
func NewHarness(t *testing.T, srv *server.Server) *Harness {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	h := &Harness{
		t:        t,
		server:   srv,
		listener: l,
		done:     make(chan struct{}),
	}

	go func() {
		defer close(h.done)
		_ = srv.Serve(l)
	}()

	t.Cleanup(func() {
		h.Close()
	})

	return h
}

// Addr returns the address the server is listening on.
func (h *Harness) Addr() string {
	return h.listener.Addr().String()
}

// Dial creates a new client connected to the test server.
func (h *Harness) Dial(opts ...client.Option) *client.Client {
	h.t.Helper()

	c, err := client.Dial(h.Addr(), opts...)
	if err != nil {
		h.t.Fatalf("dial: %v", err)
	}

	h.t.Cleanup(func() {
		_ = c.Close()
	})

	return c
}

// Close shuts down the test harness.
func (h *Harness) Close() {
	_ = h.server.Close()
	<-h.done
}

// Server returns the underlying server.
func (h *Harness) Server() *server.Server {
	return h.server
}
