package server

import (
	"log/slog"
	"net"
)

// NewTestConn creates a Conn suitable for use in tests. It wraps the given
// net.Conn with a minimal server configuration using default options and the
// provided logger. This function is intended for testing middleware and other
// components that require a *Conn.
func NewTestConn(netConn net.Conn, logger *slog.Logger) *Conn {
	if logger == nil {
		logger = slog.Default()
	}
	srv := New(WithLogger(logger))
	return newConn(netConn, srv)
}
