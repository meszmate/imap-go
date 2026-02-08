// Package server implements an IMAP server.
//
// The server uses an extensible command dispatch system and supports
// middleware, custom extensions, and pluggable authentication.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	imap "github.com/meszmate/imap-go"
)

// Server is an IMAP server.
type Server struct {
	options    *Options
	dispatcher *Dispatcher
	listeners  []net.Listener

	mu         sync.Mutex
	conns      map[*Conn]struct{}
	connCount  atomic.Int64
	shutdown   chan struct{}
	isShutdown bool
}

// New creates a new IMAP server with the given options.
func New(opts ...Option) *Server {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	srv := &Server{
		options:    options,
		dispatcher: NewDispatcher(),
		conns:      make(map[*Conn]struct{}),
		shutdown:   make(chan struct{}),
	}

	// Register built-in command handlers
	srv.registerBuiltinHandlers()

	return srv
}

// Handle registers a command handler.
func (srv *Server) Handle(name string, handler CommandHandler) {
	srv.dispatcher.Register(name, handler)
}

// HandleFunc registers a command handler function.
func (srv *Server) HandleFunc(name string, fn CommandHandlerFunc) {
	srv.dispatcher.RegisterFunc(name, fn)
}

// WrapHandler wraps an existing command handler with a wrapper function.
func (srv *Server) WrapHandler(name string, wrapper func(CommandHandler) CommandHandler) {
	srv.dispatcher.Wrap(name, wrapper)
}

// Capabilities returns the capabilities for a connection.
func (srv *Server) Capabilities(c *Conn) []imap.Cap {
	caps := srv.options.Caps.Clone()

	// Add STARTTLS if enabled and not already using TLS
	if srv.options.EnableStartTLS && !c.IsTLS() {
		caps.Add(imap.CapStartTLS)
	}

	// Add LOGINDISABLED if not using TLS and insecure auth is not allowed
	if !c.IsTLS() && !srv.options.AllowInsecureAuth {
		caps.Add(imap.CapLogindisabled)
	}

	return caps.All()
}

// Serve accepts connections on the listener and serves each one.
func (srv *Server) Serve(l net.Listener) error {
	srv.mu.Lock()
	if srv.isShutdown {
		srv.mu.Unlock()
		return errors.New("server is shut down")
	}
	srv.listeners = append(srv.listeners, l)
	srv.mu.Unlock()

	defer func() {
		srv.mu.Lock()
		for i, listener := range srv.listeners {
			if listener == l {
				srv.listeners = append(srv.listeners[:i], srv.listeners[i+1:]...)
				break
			}
		}
		srv.mu.Unlock()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-srv.shutdown:
				return nil
			default:
			}
			srv.options.Logger.Error("accept error", "error", err)
			continue
		}

		if srv.options.MaxConnections > 0 && int(srv.connCount.Load()) >= srv.options.MaxConnections {
			srv.options.Logger.Warn("max connections reached, rejecting", "remote", conn.RemoteAddr())
			conn.Close()
			continue
		}

		go srv.handleConn(conn)
	}
}

// ListenAndServe listens on the given address and serves.
func (srv *Server) ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return srv.Serve(l)
}

// ListenAndServeTLS listens on the given address with TLS and serves.
func (srv *Server) ListenAndServeTLS(addr string, config *tls.Config) error {
	if config == nil {
		config = srv.options.TLSConfig
	}
	if config == nil {
		return errors.New("TLS config required")
	}

	l, err := tls.Listen("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("TLS listen: %w", err)
	}
	return srv.Serve(l)
}

// Shutdown gracefully shuts down the server.
func (srv *Server) Shutdown(ctx context.Context) error {
	srv.mu.Lock()
	srv.isShutdown = true
	close(srv.shutdown)

	// Close all listeners
	for _, l := range srv.listeners {
		l.Close()
	}
	srv.mu.Unlock()

	// Close all connections
	srv.mu.Lock()
	for c := range srv.conns {
		c.WriteBYE("server shutting down")
		c.Close()
	}
	srv.mu.Unlock()

	return nil
}

// Close immediately closes the server and all connections.
func (srv *Server) Close() error {
	return srv.Shutdown(context.Background())
}

// Options returns the server options.
func (srv *Server) Options() *Options {
	return srv.options
}

// Logger returns the server logger.
func (srv *Server) Logger() *slog.Logger {
	return srv.options.Logger
}

// Dispatcher returns the command dispatcher.
func (srv *Server) Dispatcher() *Dispatcher {
	return srv.dispatcher
}

func (srv *Server) handleConn(netConn net.Conn) {
	c := newConn(netConn, srv)

	srv.mu.Lock()
	srv.conns[c] = struct{}{}
	srv.mu.Unlock()
	srv.connCount.Add(1)

	defer func() {
		srv.mu.Lock()
		delete(srv.conns, c)
		srv.mu.Unlock()
		srv.connCount.Add(-1)
		c.Close()
	}()

	// Create session
	if srv.options.NewSession != nil {
		session, err := srv.options.NewSession(c)
		if err != nil {
			c.logger.Error("failed to create session", "error", err)
			c.WriteBYE("internal server error")
			return
		}
		c.session = session
	}

	c.serve()
}

// RegisterBuiltinFunc is the function called to register built-in handlers.
// It is set by the commands package's init function.
var RegisterBuiltinFunc func(srv *Server)

// registerBuiltinHandlers registers all built-in command handlers.
func (srv *Server) registerBuiltinHandlers() {
	if RegisterBuiltinFunc != nil {
		RegisterBuiltinFunc(srv)
	}
}
