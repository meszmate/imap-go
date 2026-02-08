// Package quota implements the QUOTA extension (RFC 9208).
//
// QUOTA allows clients to query and manage storage quotas. It supports
// GETQUOTA (query a specific quota root), GETQUOTAROOT (find quota roots
// for a mailbox), and SETQUOTA (set resource limits on a quota root).
package quota

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionQuota is an optional interface for sessions that support QUOTA commands.
type SessionQuota interface {
	// GetQuota returns quota data for the named quota root.
	GetQuota(root string) (*imap.QuotaData, error)

	// GetQuotaRoot returns the quota roots for the named mailbox, along
	// with the quota data for each root.
	GetQuotaRoot(mailbox string) (*imap.QuotaRootData, []*imap.QuotaData, error)

	// SetQuota sets the resource limits for the named quota root.
	SetQuota(root string, resources []imap.QuotaResourceData) (*imap.QuotaData, error)
}

// Extension implements the QUOTA extension (RFC 9208).
type Extension struct {
	extension.BaseExtension
}

// New creates a new QUOTA extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName: "QUOTA",
			ExtCapabilities: []imap.Cap{
				imap.CapQuota,
				imap.CapQuotaResStorage,
				imap.CapQuotaResMessage,
			},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"GETQUOTA":     server.CommandHandlerFunc(handleGetQuota),
		"GETQUOTAROOT": server.CommandHandlerFunc(handleGetQuotaRoot),
		"SETQUOTA":     server.CommandHandlerFunc(handleSetQuota),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionQuota)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// writeQuotaResponse writes a QUOTA untagged response for a single quota root.
func writeQuotaResponse(conn *server.Conn, data *imap.QuotaData) {
	conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("QUOTA").SP().AString(data.Root).SP().BeginList()
		for i, res := range data.Resources {
			if i > 0 {
				enc.SP()
			}
			enc.Atom(string(res.Name)).SP().
				Number64(uint64(res.Usage)).SP().
				Number64(uint64(res.Limit))
		}
		enc.EndList().CRLF()
	})
}

// handleGetQuota handles the GETQUOTA command.
//
// Command syntax: GETQUOTA quota-root
// Response:       * QUOTA root (resource usage limit ...)
func handleGetQuota(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionQuota)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GETQUOTA not supported")
		return nil
	}

	root, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected quota root name")
		return nil
	}

	data, err := sess.GetQuota(root)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GETQUOTA failed: %v", err))
		return nil
	}

	writeQuotaResponse(ctx.Conn, data)
	ctx.Conn.WriteOK(ctx.Tag, "GETQUOTA completed")
	return nil
}

// handleGetQuotaRoot handles the GETQUOTAROOT command.
//
// Command syntax: GETQUOTAROOT mailbox
// Response:       * QUOTAROOT mailbox root1 root2 ...
//
//	* QUOTA root1 (resource usage limit ...)
//	* QUOTA root2 (resource usage limit ...)
func handleGetQuotaRoot(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionQuota)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GETQUOTAROOT not supported")
		return nil
	}

	mailbox, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	rootData, quotas, err := sess.GetQuotaRoot(mailbox)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GETQUOTAROOT failed: %v", err))
		return nil
	}

	// Write QUOTAROOT response
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("QUOTAROOT").SP().MailboxName(rootData.Mailbox)
		for _, root := range rootData.Roots {
			enc.SP().AString(root)
		}
		enc.CRLF()
	})

	// Write QUOTA response for each root
	for _, q := range quotas {
		writeQuotaResponse(ctx.Conn, q)
	}

	ctx.Conn.WriteOK(ctx.Tag, "GETQUOTAROOT completed")
	return nil
}

// handleSetQuota handles the SETQUOTA command.
//
// Command syntax: SETQUOTA quota-root (resource limit ...)
// Response:       * QUOTA root (resource usage limit ...)
func handleSetQuota(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionQuota)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "SETQUOTA not supported")
		return nil
	}

	dec := ctx.Decoder

	root, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected quota root name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected resource list")
		return nil
	}

	// Parse resource limits list: (resource limit ...)
	var resources []imap.QuotaResourceData
	if err := dec.ReadList(func() error {
		resName, err := dec.ReadAtom()
		if err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return err
		}
		limit, err := dec.ReadNumber64()
		if err != nil {
			return err
		}
		resources = append(resources, imap.QuotaResourceData{
			Name:  imap.QuotaResource(resName),
			Limit: int64(limit),
		})
		return nil
	}); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Invalid resource list")
		return nil
	}

	data, err := sess.SetQuota(root, resources)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("SETQUOTA failed: %v", err))
		return nil
	}

	writeQuotaResponse(ctx.Conn, data)
	ctx.Conn.WriteOK(ctx.Tag, "SETQUOTA completed")
	return nil
}
