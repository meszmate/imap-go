package namespace

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Extension implements the NAMESPACE IMAP extension (RFC 2342).
// NAMESPACE allows the client to discover the prefixes and delimiters
// used by the server for personal, other users', and shared namespaces.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new NAMESPACE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "NAMESPACE",
			ExtCapabilities: []imap.Cap{imap.CapNamespace},
		},
	}
}

// CommandHandlers returns the NAMESPACE command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandNamespace: handleNamespace(),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionNamespace interface that sessions
// must implement to support the NAMESPACE command.
func (e *Extension) SessionExtension() interface{} {
	return (*server.SessionNamespace)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleNamespace returns the command handler function for the NAMESPACE command.
func handleNamespace() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		// The session must implement SessionNamespace
		sessNS, ok := ctx.Session.(server.SessionNamespace)
		if !ok {
			return imap.ErrNo("NAMESPACE not supported")
		}

		data, err := sessNS.Namespace()
		if err != nil {
			return err
		}

		// Write untagged NAMESPACE response
		ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("NAMESPACE").SP()
			writeNamespaceList(enc, data.Personal)
			enc.SP()
			writeNamespaceList(enc, data.Other)
			enc.SP()
			writeNamespaceList(enc, data.Shared)
			enc.CRLF()
		})

		ctx.Conn.WriteOK(ctx.Tag, "NAMESPACE completed")
		return nil
	}
}

// writeNamespaceList writes a list of namespace descriptors, or NIL if empty.
func writeNamespaceList(enc *wire.Encoder, descriptors []imap.NamespaceDescriptor) {
	if len(descriptors) == 0 {
		enc.Nil()
		return
	}
	enc.BeginList()
	for i, ns := range descriptors {
		if i > 0 {
			enc.SP()
		}
		enc.BeginList()
		enc.QuotedString(ns.Prefix)
		enc.SP()
		if ns.Delim == 0 {
			enc.Nil()
		} else {
			enc.QuotedString(string(ns.Delim))
		}
		enc.EndList()
	}
	enc.EndList()
}
