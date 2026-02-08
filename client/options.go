package client

import (
	"crypto/tls"
	"log/slog"
	"time"
)

// Option is a functional option for configuring the client.
type Option func(*Options)

// Options holds all client configuration.
type Options struct {
	// TLSConfig is the TLS configuration for TLS connections.
	TLSConfig *tls.Config

	// Logger is the structured logger.
	Logger *slog.Logger

	// ReadTimeout is the timeout for reading a single response.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing a command.
	WriteTimeout time.Duration

	// IdleTimeout is the timeout for IDLE commands.
	IdleTimeout time.Duration

	// UnilateralDataHandler handles unsolicited server responses.
	UnilateralDataHandler *UnilateralDataHandler

	// DebugLog enables wire-level protocol logging.
	DebugLog bool
}

// UnilateralDataHandler handles unsolicited server data.
type UnilateralDataHandler struct {
	Expunge func(seqNum uint32)
	Exists  func(count uint32)
	Recent  func(count uint32)
	Fetch   func(seqNum uint32, flags []string)
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		Logger:      slog.Default(),
		ReadTimeout: 30 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		IdleTimeout: 30 * time.Minute,
	}
}

// WithTLSConfig sets the TLS configuration.
func WithTLSConfig(config *tls.Config) Option {
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

// WithUnilateralDataHandler sets the handler for unsolicited data.
func WithUnilateralDataHandler(h *UnilateralDataHandler) Option {
	return func(o *Options) {
		o.UnilateralDataHandler = h
	}
}

// WithDebugLog enables wire-level protocol logging.
func WithDebugLog(enable bool) Option {
	return func(o *Options) {
		o.DebugLog = enable
	}
}
