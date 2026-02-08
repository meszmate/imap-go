package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Store returns a handler for the STORE command.
// STORE alters flags associated with messages in the mailbox.
func Store() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		// Read sequence set
		seqSetStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid sequence set")
		}

		var numSet imap.NumSet
		if ctx.NumKind == server.NumKindUID {
			uidSet, err := imap.ParseUIDSet(seqSetStr)
			if err != nil {
				return imap.ErrBad("invalid UID set")
			}
			numSet = uidSet
		} else {
			seqSet, err := imap.ParseSeqSet(seqSetStr)
			if err != nil {
				return imap.ErrBad("invalid sequence set")
			}
			numSet = seqSet
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing store action")
		}

		// Read store action (FLAGS, FLAGS.SILENT, +FLAGS, +FLAGS.SILENT, -FLAGS, -FLAGS.SILENT)
		actionStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid store action")
		}

		storeFlags := &imap.StoreFlags{}
		upper := strings.ToUpper(actionStr)

		switch {
		case strings.HasPrefix(upper, "+FLAGS"):
			storeFlags.Action = imap.StoreFlagsAdd
		case strings.HasPrefix(upper, "-FLAGS"):
			storeFlags.Action = imap.StoreFlagsDel
		case strings.HasPrefix(upper, "FLAGS"):
			storeFlags.Action = imap.StoreFlagsSet
		default:
			return imap.ErrBad("invalid store action: " + actionStr)
		}

		if strings.HasSuffix(upper, ".SILENT") {
			storeFlags.Silent = true
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing flags")
		}

		// Read flags
		flagStrs, err := ctx.Decoder.ReadFlags()
		if err != nil {
			return imap.ErrBad("invalid flags")
		}

		for _, f := range flagStrs {
			storeFlags.Flags = append(storeFlags.Flags, imap.Flag(f))
		}

		options := &imap.StoreOptions{}

		w := server.NewFetchWriter(ctx.Conn.Encoder())
		if err := ctx.Session.Store(w, numSet, storeFlags, options); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "STORE completed")
		return nil
	}
}
