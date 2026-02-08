package server

import (
	"crypto/tls"
	"log/slog"
	"time"

	imap "github.com/meszmate/imap-go"
)

// Option is a functional option for configuring the server.
type Option func(*Options)

// Options holds all server configuration.
type Options struct {
	// TLSConfig is the TLS configuration for implicit TLS connections.
	TLSConfig *tls.Config

	// Caps is the set of capabilities to advertise.
	Caps *imap.CapSet

	// Logger is the structured logger.
	Logger *slog.Logger

	// NewSession is called when a new connection is established.
	// It must return a Session implementation.
	NewSession func(conn *Conn) (Session, error)

	// MaxLiteralSize is the maximum size of a literal that the server will accept.
	// 0 means no limit.
	MaxLiteralSize int64

	// ReadTimeout is the timeout for reading a single command.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing a response.
	WriteTimeout time.Duration

	// IdleTimeout is the timeout for IDLE commands.
	IdleTimeout time.Duration

	// MaxConnections is the maximum number of concurrent connections.
	// 0 means no limit.
	MaxConnections int

	// GreetingText is the text sent in the initial greeting.
	GreetingText string

	// AllowInsecureAuth allows authentication without TLS.
	AllowInsecureAuth bool

	// EnableStartTLS enables STARTTLS support.
	EnableStartTLS bool

	// InsecureSkipVerify disables TLS certificate verification (for testing).
	InsecureSkipVerify bool
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		Caps:         NewDefaultCapSet(),
		Logger:       slog.Default(),
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		IdleTimeout:  30 * time.Minute,
		GreetingText: "IMAP server ready",
	}
}

// NewDefaultCapSet returns a CapSet with the default capabilities.
func NewDefaultCapSet() *imap.CapSet {
	return imap.NewCapSet(
		imap.CapIMAP4rev1,
		imap.CapIdle,
		imap.CapLiteralPlus,
	)
}

// WithTLS configures TLS for the server.
func WithTLS(config *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = config
	}
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithNewSession sets the session factory.
func WithNewSession(fn func(conn *Conn) (Session, error)) Option {
	return func(o *Options) {
		o.NewSession = fn
	}
}

// WithMaxLiteralSize sets the maximum literal size.
func WithMaxLiteralSize(size int64) Option {
	return func(o *Options) {
		o.MaxLiteralSize = size
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.ReadTimeout = d
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.WriteTimeout = d
	}
}

// WithIdleTimeout sets the IDLE timeout.
func WithIdleTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.IdleTimeout = d
	}
}

// WithMaxConnections sets the maximum number of connections.
func WithMaxConnections(n int) Option {
	return func(o *Options) {
		o.MaxConnections = n
	}
}

// WithCapabilities adds capabilities to the server.
func WithCapabilities(caps ...imap.Cap) Option {
	return func(o *Options) {
		o.Caps.Add(caps...)
	}
}

// WithGreetingText sets the greeting text.
func WithGreetingText(text string) Option {
	return func(o *Options) {
		o.GreetingText = text
	}
}

// WithAllowInsecureAuth allows authentication without TLS.
func WithAllowInsecureAuth(allow bool) Option {
	return func(o *Options) {
		o.AllowInsecureAuth = allow
	}
}

// WithStartTLS enables STARTTLS support with the given TLS config.
func WithStartTLS(config *tls.Config) Option {
	return func(o *Options) {
		o.EnableStartTLS = true
		if o.TLSConfig == nil {
			o.TLSConfig = config
		}
	}
}
