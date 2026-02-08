// Package commands provides the built-in IMAP command handlers for the server.
//
// Each handler function returns a server.CommandHandlerFunc that implements
// the corresponding IMAP command according to RFC 9051 and RFC 3501.
//
// Importing this package automatically registers all built-in handlers
// via the init function, so that server.New() includes them by default.
package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

func init() {
	server.RegisterBuiltinFunc = RegisterAll
}

// RegisterAll registers all built-in IMAP command handlers on the given server.
func RegisterAll(srv *server.Server) {
	// Any state commands
	srv.HandleFunc(imap.CommandCapability, Capability())
	srv.HandleFunc(imap.CommandNoop, Noop())
	srv.HandleFunc(imap.CommandLogout, Logout())

	// Not authenticated state commands
	srv.HandleFunc(imap.CommandStartTLS, StartTLS())
	srv.HandleFunc(imap.CommandLogin, Login())

	// Authenticated state commands
	srv.HandleFunc(imap.CommandEnable, Enable())
	srv.HandleFunc(imap.CommandSelect, Select())
	srv.HandleFunc(imap.CommandExamine, Examine())
	srv.HandleFunc(imap.CommandCreate, Create())
	srv.HandleFunc(imap.CommandDelete, Delete())
	srv.HandleFunc(imap.CommandRename, Rename())
	srv.HandleFunc(imap.CommandSubscribe, Subscribe())
	srv.HandleFunc(imap.CommandUnsubscribe, Unsubscribe())
	srv.HandleFunc(imap.CommandList, List())
	srv.HandleFunc(imap.CommandLsub, Lsub())
	srv.HandleFunc(imap.CommandStatus, Status())
	srv.HandleFunc(imap.CommandAppend, Append())
	srv.HandleFunc(imap.CommandIdle, Idle())

	// Selected state commands
	srv.HandleFunc(imap.CommandClose, Close())
	srv.HandleFunc(imap.CommandUnselect, Unselect())
	srv.HandleFunc(imap.CommandExpunge, Expunge())
	srv.HandleFunc(imap.CommandSearch, Search())
	srv.HandleFunc(imap.CommandFetch, Fetch())
	srv.HandleFunc(imap.CommandStore, Store())
	srv.HandleFunc(imap.CommandCopy, Copy())
}
