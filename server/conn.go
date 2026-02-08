package server

import (
	"crypto/tls"
	"log/slog"
	"net"
	"sync"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/state"
	"github.com/meszmate/imap-go/wire"
)

// Conn represents a single IMAP client connection.
type Conn struct {
	netConn net.Conn
	server  *Server
	session Session

	decoder *wire.Decoder
	encoder *ResponseEncoder

	state   *state.Machine
	enabled *imap.CapSet

	logger *slog.Logger

	mu       sync.Mutex
	isTLS    bool
	mailbox  string
	readOnly bool
	closed   bool
}

// newConn creates a new connection.
func newConn(netConn net.Conn, srv *Server) *Conn {
	enc := wire.NewEncoder(netConn)
	c := &Conn{
		netConn: netConn,
		server:  srv,
		decoder: wire.NewDecoder(netConn),
		encoder: NewResponseEncoder(enc),
		state:   state.New(imap.ConnStateNotAuthenticated),
		enabled: imap.NewCapSet(),
		logger:  srv.options.Logger.With("remote", netConn.RemoteAddr().String()),
	}

	_, c.isTLS = netConn.(*tls.Conn)

	return c
}

// State returns the current connection state.
func (c *Conn) State() imap.ConnState {
	return c.state.State()
}

// SetState transitions the connection to a new state.
func (c *Conn) SetState(s imap.ConnState) error {
	return c.state.Transition(s)
}

// Enabled returns the set of enabled capabilities for this connection.
func (c *Conn) Enabled() *imap.CapSet {
	return c.enabled
}

// IsTLS returns whether the connection is using TLS.
func (c *Conn) IsTLS() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isTLS
}

// Mailbox returns the currently selected mailbox name.
func (c *Conn) Mailbox() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mailbox
}

// SetMailbox sets the currently selected mailbox name.
func (c *Conn) SetMailbox(name string, readOnly bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mailbox = name
	c.readOnly = readOnly
}

// IsReadOnly returns whether the mailbox was opened read-only.
func (c *Conn) IsReadOnly() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readOnly
}

// RemoteAddr returns the remote address of the connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}

// LocalAddr returns the local address of the connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.netConn.LocalAddr()
}

// NetConn returns the underlying net.Conn.
func (c *Conn) NetConn() net.Conn {
	return c.netConn
}

// Server returns the server instance.
func (c *Conn) Server() *Server {
	return c.server
}

// Session returns the backend session.
func (c *Conn) Session() Session {
	return c.session
}

// Logger returns the connection's logger.
func (c *Conn) Logger() *slog.Logger {
	return c.logger
}

// Close closes the connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	if c.session != nil {
		_ = c.session.Close()
	}
	return c.netConn.Close()
}

// WriteOK writes a tagged OK response.
func (c *Conn) WriteOK(tag, text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(tag, "OK", "", text)
	})
}

// WriteOKCode writes a tagged OK response with a response code.
func (c *Conn) WriteOKCode(tag, code, text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(tag, "OK", code, text)
	})
}

// WriteNO writes a tagged NO response.
func (c *Conn) WriteNO(tag, text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(tag, "NO", "", text)
	})
}

// WriteBAD writes a tagged BAD response.
func (c *Conn) WriteBAD(tag, text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(tag, "BAD", "", text)
	})
}

// WriteBYE writes an untagged BYE response.
func (c *Conn) WriteBYE(text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse("*", "BYE", "", text)
	})
}

// WriteCapabilities writes an untagged CAPABILITY response.
func (c *Conn) WriteCapabilities() {
	caps := c.server.Capabilities(c)
	capStrs := make([]string, len(caps))
	for i, cap := range caps {
		capStrs[i] = string(cap)
	}

	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("CAPABILITY")
		for _, cap := range capStrs {
			enc.SP().Atom(cap)
		}
		enc.CRLF()
	})
}

// WriteContinuation writes a continuation request.
func (c *Conn) WriteContinuation(text string) {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.ContinuationRequest(text)
	})
}

// Encoder returns the connection's response encoder.
func (c *Conn) Encoder() *ResponseEncoder {
	return c.encoder
}

// Decoder returns the connection's wire decoder.
func (c *Conn) Decoder() *wire.Decoder {
	return c.decoder
}

// writeGreeting writes the initial server greeting.
func (c *Conn) writeGreeting() {
	c.encoder.Encode(func(enc *wire.Encoder) {
		enc.StatusResponse("*", "OK", "", c.server.options.GreetingText)
	})
}

// UpgradeTLS upgrades the connection to TLS.
func (c *Conn) UpgradeTLS(config *tls.Config) error {
	tlsConn := tls.Server(c.netConn, config)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	c.mu.Lock()
	c.netConn = tlsConn
	c.isTLS = true
	c.mu.Unlock()

	// Re-create decoder and encoder with the new connection
	c.decoder = wire.NewDecoder(tlsConn)
	c.encoder = NewResponseEncoder(wire.NewEncoder(tlsConn))

	return nil
}

// serve is the main connection loop.
func (c *Conn) serve() {
	defer func() { _ = c.Close() }()

	c.writeGreeting()

	for {
		if err := c.readAndHandle(); err != nil {
			c.logger.Debug("connection error", "error", err)
			return
		}

		if c.State() == imap.ConnStateLogout {
			return
		}
	}
}

// readAndHandle reads and dispatches a single command.
func (c *Conn) readAndHandle() error {
	line, err := c.decoder.ReadLine()
	if err != nil {
		return err
	}

	tag, name, rest, err := parseLine(line)
	if err != nil {
		c.WriteBAD("*", err.Error())
		return nil
	}

	c.logger.Debug("command", "tag", tag, "name", name)

	return c.server.dispatch(c, tag, name, rest)
}
