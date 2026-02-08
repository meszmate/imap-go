// Package extension provides the IMAP extension/plugin system.
//
// Extensions can add new commands, modify existing command behavior,
// advertise capabilities, and require session interfaces.
package extension

import (
	imap "github.com/meszmate/imap-go"
)

// Extension is the base interface for all IMAP extensions.
type Extension interface {
	// Name returns the unique name of the extension.
	Name() string
	// Capabilities returns the capabilities this extension provides.
	Capabilities() []imap.Cap
	// Dependencies returns the names of extensions this one depends on.
	Dependencies() []string
}

// ServerExtension extends the IMAP server with new functionality.
type ServerExtension interface {
	Extension

	// CommandHandlers returns new command handlers to register.
	// The map key is the command name (uppercase).
	CommandHandlers() map[string]interface{}

	// WrapHandler wraps an existing command handler.
	// Return nil to not wrap the handler.
	WrapHandler(name string, handler interface{}) interface{}

	// SessionExtension returns the required session extension interface, or nil.
	// The server will check that sessions implement this interface.
	SessionExtension() interface{}

	// OnEnabled is called when a client enables this extension via ENABLE.
	OnEnabled(connID string) error
}

// ClientExtension extends the IMAP client with new functionality.
type ClientExtension interface {
	Extension

	// ResponseHandlers returns handlers for untagged responses.
	ResponseHandlers() map[string]interface{}

	// ResponseCodeHandlers returns handlers for response codes.
	ResponseCodeHandlers() map[string]interface{}

	// WrapResponseHandler wraps an existing response handler.
	WrapResponseHandler(name string, handler interface{}) interface{}
}

// BaseExtension provides a default implementation of Extension.
type BaseExtension struct {
	ExtName         string
	ExtCapabilities []imap.Cap
	ExtDependencies []string
}

// Name implements Extension.
func (e *BaseExtension) Name() string { return e.ExtName }

// Capabilities implements Extension.
func (e *BaseExtension) Capabilities() []imap.Cap { return e.ExtCapabilities }

// Dependencies implements Extension.
func (e *BaseExtension) Dependencies() []string { return e.ExtDependencies }
