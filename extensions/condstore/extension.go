// Package condstore implements the CONDSTORE extension (RFC 7162).
//
// CONDSTORE adds conditional STORE operations and per-message modification
// sequence numbers (MODSEQ). It modifies FETCH, STORE, SELECT, and SEARCH
// to handle MODSEQ values.
//
// The core types already support CONDSTORE fields: FetchOptions.ModSeq,
// FetchOptions.ChangedSince, StoreOptions.UnchangedSince, SelectOptions.CondStore,
// and SearchCriteria.ModSeq. This extension adds protocol-level parsing for
// UNCHANGEDSINCE (STORE), CHANGEDSINCE (FETCH), and CONDSTORE (SELECT/EXAMINE).
package condstore

import (
	"strconv"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionCondStore is the session interface for CONDSTORE support.
// Backends implement this to handle conditional STORE operations that use
// the UNCHANGEDSINCE modifier, returning per-message MODSEQ values.
type SessionCondStore interface {
	// StoreConditional stores flags on messages conditionally based on MODSEQ.
	// The options.UnchangedSince field specifies the MODSEQ threshold;
	// messages modified since that value must not be updated.
	StoreConditional(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error
}

// Extension implements the CONDSTORE extension (RFC 7162).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CONDSTORE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CONDSTORE",
			ExtCapabilities: []imap.Cap{imap.CapCondStore},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// CONDSTORE modifies existing commands (FETCH, STORE, SELECT, SEARCH) rather
// than adding new ones, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps existing command handlers to add CONDSTORE parsing.
// It wraps STORE (UNCHANGEDSINCE), FETCH (CHANGEDSINCE), and SELECT/EXAMINE
// (CONDSTORE parameter) commands.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	h, ok := handler.(server.CommandHandlerFunc)
	if !ok {
		ch, ok2 := handler.(server.CommandHandler)
		if !ok2 {
			return nil
		}
		h = ch.Handle
	}

	switch name {
	case "STORE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleCondstoreStore(ctx, h)
		})
	case "FETCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleCondstoreFetch(ctx, h)
		})
	case "SELECT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleCondstoreSelect(ctx, false)
		})
	case "EXAMINE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleCondstoreSelect(ctx, true)
		})
	}
	return nil
}

// SessionExtension returns a typed nil pointer to SessionCondStore, indicating
// that sessions should implement this interface for full CONDSTORE support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionCondStore)(nil)
}

// OnEnabled is called when a client enables CONDSTORE via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleCondstoreStore wraps the STORE command to parse (UNCHANGEDSINCE <modseq>).
//
// Format: STORE <seqset> (UNCHANGEDSINCE <modseq>) <action> <flags>
func handleCondstoreStore(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	// Read sequence set
	seqSetStr, err := dec.ReadAtom()
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

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing store action")
	}

	// Peek for '(' to check for UNCHANGEDSINCE modifier
	options := &imap.StoreOptions{}
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end of command")
	}

	if b == '(' {
		// Parse (UNCHANGEDSINCE <modseq>)
		if err := dec.ExpectByte('('); err != nil {
			return imap.ErrBad("invalid modifier")
		}
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid modifier name")
		}
		if !strings.EqualFold(atom, "UNCHANGEDSINCE") {
			return imap.ErrBad("unknown store modifier: " + atom)
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing UNCHANGEDSINCE value")
		}
		modseq, err := dec.ReadNumber64()
		if err != nil {
			return imap.ErrBad("invalid UNCHANGEDSINCE value")
		}
		if err := dec.ExpectByte(')'); err != nil {
			return imap.ErrBad("missing closing paren for modifier")
		}
		options.UnchangedSince = modseq

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing store action")
		}
	}

	// Read store action (FLAGS, FLAGS.SILENT, +FLAGS, +FLAGS.SILENT, -FLAGS, -FLAGS.SILENT)
	actionStr, err := dec.ReadAtom()
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

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing flags")
	}

	// Read flags
	flagStrs, err := dec.ReadFlags()
	if err != nil {
		return imap.ErrBad("invalid flags")
	}

	for _, f := range flagStrs {
		storeFlags.Flags = append(storeFlags.Flags, imap.Flag(f))
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())

	if options.UnchangedSince > 0 {
		sess, ok := ctx.Session.(SessionCondStore)
		if !ok {
			return ctx.Session.Store(w, numSet, storeFlags, options)
		}
		if err := sess.StoreConditional(w, numSet, storeFlags, options); err != nil {
			return err
		}
	} else {
		if err := ctx.Session.Store(w, numSet, storeFlags, options); err != nil {
			return err
		}
	}

	ctx.Conn.WriteOK(ctx.Tag, "STORE completed")
	return nil
}

// handleCondstoreFetch wraps the FETCH command to parse (CHANGEDSINCE <modseq>).
//
// Format: FETCH <seqset> <items> (CHANGEDSINCE <modseq>)
func handleCondstoreFetch(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing arguments")
	}

	dec := ctx.Decoder

	// Read sequence set
	seqSetStr, err := dec.ReadAtom()
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

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing fetch items")
	}

	// Parse fetch items
	options, err := ParseFetchItems(dec)
	if err != nil {
		return imap.ErrBad("invalid fetch items: " + err.Error())
	}

	// Check for (CHANGEDSINCE <modseq>) modifier after fetch items
	if err := dec.ReadSP(); err == nil {
		b, err := dec.PeekByte()
		if err == nil && b == '(' {
			if err := dec.ExpectByte('('); err != nil {
				return imap.ErrBad("invalid modifier")
			}
			atom, err := dec.ReadAtom()
			if err != nil {
				return imap.ErrBad("invalid modifier name")
			}
			if !strings.EqualFold(atom, "CHANGEDSINCE") {
				return imap.ErrBad("unknown fetch modifier: " + atom)
			}
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing CHANGEDSINCE value")
			}
			modseq, err := dec.ReadNumber64()
			if err != nil {
				return imap.ErrBad("invalid CHANGEDSINCE value")
			}
			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for modifier")
			}
			options.ChangedSince = modseq
			options.ModSeq = true
		}
	}

	// Always include UID for UID FETCH
	if ctx.NumKind == server.NumKindUID {
		options.UID = true
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	if err := ctx.Session.Fetch(w, numSet, options); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "FETCH completed")
	return nil
}

// ParseFetchItems parses FETCH item specifications.
func ParseFetchItems(dec *wire.Decoder) (*imap.FetchOptions, error) {
	options := &imap.FetchOptions{}

	b, err := dec.PeekByte()
	if err != nil {
		return nil, err
	}

	if b == '(' {
		if err := dec.ReadList(func() error {
			return ParseSingleFetchItem(dec, options)
		}); err != nil {
			return nil, err
		}
	} else {
		if err := ParseSingleFetchItem(dec, options); err != nil {
			return nil, err
		}
	}

	return options, nil
}

// ParseSingleFetchItem parses a single FETCH item or macro.
func ParseSingleFetchItem(dec *wire.Decoder, options *imap.FetchOptions) error {
	item, err := dec.ReadAtom()
	if err != nil {
		return err
	}

	upper := strings.ToUpper(item)
	switch {
	case upper == "ALL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
	case upper == "FAST":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
	case upper == "FULL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
		options.BodyStructure = true
	case upper == "ENVELOPE":
		options.Envelope = true
	case upper == "FLAGS":
		options.Flags = true
	case upper == "INTERNALDATE":
		options.InternalDate = true
	case upper == "RFC822.SIZE":
		options.RFC822Size = true
	case upper == "UID":
		options.UID = true
	case upper == "BODYSTRUCTURE":
		options.BodyStructure = true
	case upper == "MODSEQ":
		options.ModSeq = true
	case upper == "PREVIEW":
		options.Preview = true
	case upper == "EMAILID":
		options.EmailID = true
	case upper == "THREADID":
		options.ThreadID = true
	case upper == "SAVEDATE":
		options.SaveDate = true

	// BODY with bracket embedded in atom ([ is an atom char)
	case strings.HasPrefix(upper, "BODY.PEEK["):
		section, err := parseFetchBodySectionFromAtom(dec, item, true)
		if err != nil {
			return err
		}
		options.BodySection = append(options.BodySection, section)
	case strings.HasPrefix(upper, "BODY["):
		section, err := parseFetchBodySectionFromAtom(dec, item, false)
		if err != nil {
			return err
		}
		options.BodySection = append(options.BodySection, section)
	case upper == "BODY.PEEK":
		b, err := dec.PeekByte()
		if err == nil && b == '[' {
			section, err := ParseFetchBodySection(dec, true)
			if err != nil {
				return err
			}
			options.BodySection = append(options.BodySection, section)
		}
	case upper == "BODY":
		b, err := dec.PeekByte()
		if err == nil && b == '[' {
			section, err := ParseFetchBodySection(dec, false)
			if err != nil {
				return err
			}
			options.BodySection = append(options.BodySection, section)
		} else {
			options.BodyStructure = true
		}

	// BINARY items (RFC 3516)
	case strings.HasPrefix(upper, "BINARY.SIZE["):
		part := ParseBinaryPart(item[len("BINARY.SIZE["):])
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		options.BinarySizeSection = append(options.BinarySizeSection, part)
	case strings.HasPrefix(upper, "BINARY.PEEK["):
		section := ParseBinaryItemFromAtom(item, "BINARY.PEEK[", true)
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		section.Partial = ConsumePartial(dec)
		options.BinarySection = append(options.BinarySection, section)
	case strings.HasPrefix(upper, "BINARY["):
		section := ParseBinaryItemFromAtom(item, "BINARY[", false)
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		section.Partial = ConsumePartial(dec)
		options.BinarySection = append(options.BinarySection, section)

	case upper == "RFC822":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{})
	case upper == "RFC822.HEADER":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "HEADER",
			Peek:      true,
		})
	case upper == "RFC822.TEXT":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "TEXT",
		})
	}

	return nil
}

// parseFetchBodySectionFromAtom handles BODY[section or BODY.PEEK[section
// where [ was included in the atom by ReadAtom.
func parseFetchBodySectionFromAtom(dec *wire.Decoder, item string, peek bool) (*imap.FetchItemBodySection, error) {
	bracketIdx := strings.IndexByte(item, '[')
	sectionStr := strings.ToUpper(item[bracketIdx+1:])

	section := &imap.FetchItemBodySection{Peek: peek}

	switch {
	case sectionStr == "":
		// empty section — will read ] next
	case sectionStr == "HEADER":
		section.Specifier = "HEADER"
	case sectionStr == "TEXT":
		section.Specifier = "TEXT"
	case sectionStr == "MIME":
		section.Specifier = "MIME"
	case strings.HasPrefix(sectionStr, "HEADER.FIELDS.NOT"):
		section.Specifier = "HEADER.FIELDS.NOT"
		section.NotFields = true
		if err := dec.ReadSP(); err != nil {
			return nil, err
		}
		fields, err := ReadFieldList(dec)
		if err != nil {
			return nil, err
		}
		section.Fields = fields
	case strings.HasPrefix(sectionStr, "HEADER.FIELDS"):
		section.Specifier = "HEADER.FIELDS"
		if err := dec.ReadSP(); err != nil {
			return nil, err
		}
		fields, err := ReadFieldList(dec)
		if err != nil {
			return nil, err
		}
		section.Fields = fields
	}

	if err := dec.ExpectByte(']'); err != nil {
		return nil, err
	}

	// Check for partial <offset.count>
	b, err := dec.PeekByte()
	if err == nil && b == '<' {
		for {
			ch, err := dec.PeekByte()
			if err != nil {
				break
			}
			if ch == '>' {
				_ = dec.ExpectByte('>')
				break
			}
			_, _ = dec.ReadAtom()
		}
	}

	return section, nil
}

// ParseBinaryPart parses a MIME part string like "1.2" into []int{1, 2}.
func ParseBinaryPart(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		result = append(result, n)
	}
	return result
}

// ParseBinaryItemFromAtom builds a FetchItemBinarySection from the atom string.
func ParseBinaryItemFromAtom(item, prefix string, peek bool) *imap.FetchItemBinarySection {
	sectionStr := item[len(prefix):]
	return &imap.FetchItemBinarySection{
		Part: ParseBinaryPart(sectionStr),
		Peek: peek,
	}
}

// ConsumePartial consumes a <offset.count> partial specifier if present.
func ConsumePartial(dec *wire.Decoder) *imap.SectionPartial {
	b, err := dec.PeekByte()
	if err != nil || b != '<' {
		return nil
	}
	_ = dec.ExpectByte('<')
	atom, err := dec.ReadAtom()
	if err != nil {
		return nil
	}
	_ = dec.ExpectByte('>')

	parts := strings.SplitN(atom, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	offset, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil
	}
	count, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil
	}
	return &imap.SectionPartial{Offset: offset, Count: count}
}

// ParseFetchBodySection parses a BODY[section] or BODY.PEEK[section] specification.
func ParseFetchBodySection(dec *wire.Decoder, peek bool) (*imap.FetchItemBodySection, error) {
	section := &imap.FetchItemBodySection{
		Peek: peek,
	}

	if err := dec.ExpectByte('['); err != nil {
		return nil, err
	}

	b, err := dec.PeekByte()
	if err != nil {
		return nil, err
	}

	if b != ']' {
		spec, err := dec.ReadAtom()
		if err != nil {
			return nil, err
		}

		upper := strings.ToUpper(spec)
		switch {
		case upper == "HEADER":
			section.Specifier = "HEADER"
		case upper == "TEXT":
			section.Specifier = "TEXT"
		case upper == "MIME":
			section.Specifier = "MIME"
		case strings.HasPrefix(upper, "HEADER.FIELDS.NOT"):
			section.Specifier = "HEADER.FIELDS.NOT"
			section.NotFields = true
			if err := dec.ReadSP(); err != nil {
				return nil, err
			}
			fields, err := ReadFieldList(dec)
			if err != nil {
				return nil, err
			}
			section.Fields = fields
		case strings.HasPrefix(upper, "HEADER.FIELDS"):
			section.Specifier = "HEADER.FIELDS"
			if err := dec.ReadSP(); err != nil {
				return nil, err
			}
			fields, err := ReadFieldList(dec)
			if err != nil {
				return nil, err
			}
			section.Fields = fields
		}
	}

	if err := dec.ExpectByte(']'); err != nil {
		return nil, err
	}

	b, err = dec.PeekByte()
	if err == nil && b == '<' {
		for {
			ch, err := dec.PeekByte()
			if err != nil {
				break
			}
			if ch == '>' {
				_ = dec.ExpectByte('>')
				break
			}
			_, _ = dec.ReadAtom()
		}
	}

	return section, nil
}

// ReadFieldList reads a parenthesized list of header field names.
func ReadFieldList(dec *wire.Decoder) ([]string, error) {
	var fields []string
	if err := dec.ReadList(func() error {
		field, err := dec.ReadAString()
		if err != nil {
			return err
		}
		fields = append(fields, field)
		return nil
	}); err != nil {
		return nil, err
	}
	return fields, nil
}

// handleCondstoreSelect wraps SELECT/EXAMINE to parse (CONDSTORE) parameter.
//
// Format: SELECT <mailbox> (CONDSTORE)
func handleCondstoreSelect(ctx *server.CommandContext, readOnly bool) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing mailbox name")
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid mailbox name")
	}

	options := &imap.SelectOptions{
		ReadOnly: readOnly,
	}

	// Check for (CONDSTORE) parameter
	if err := dec.ReadSP(); err == nil {
		b, err := dec.PeekByte()
		if err == nil && b == '(' {
			if err := dec.ReadList(func() error {
				atom, err := dec.ReadAtom()
				if err != nil {
					return err
				}
				if strings.EqualFold(atom, "CONDSTORE") {
					options.CondStore = true
				}
				return nil
			}); err != nil {
				return imap.ErrBad("invalid SELECT parameters")
			}
		}
	}

	data, err := ctx.Session.Select(mailbox, options)
	if err != nil {
		return err
	}

	enc := ctx.Conn.Encoder()

	// Write FLAGS
	flagStrs := make([]string, len(data.Flags))
	for i, f := range data.Flags {
		flagStrs[i] = string(f)
	}
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("FLAGS").SP().Flags(flagStrs).CRLF()
	})

	// Write EXISTS
	enc.Encode(func(e *wire.Encoder) {
		e.NumResponse(data.NumMessages, "EXISTS")
	})

	// Write RECENT
	enc.Encode(func(e *wire.Encoder) {
		e.NumResponse(data.NumRecent, "RECENT")
	})

	// Write UIDVALIDITY
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("OK").SP()
		e.ResponseCode("UIDVALIDITY", data.UIDValidity)
		e.CRLF()
	})

	// Write UIDNEXT
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("OK").SP()
		e.ResponseCode("UIDNEXT", uint32(data.UIDNext))
		e.CRLF()
	})

	// Write PERMANENTFLAGS if present
	if len(data.PermanentFlags) > 0 {
		permFlagStrs := make([]string, len(data.PermanentFlags))
		for i, f := range data.PermanentFlags {
			permFlagStrs[i] = string(f)
		}
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.RawString("[PERMANENTFLAGS ")
			e.Flags(permFlagStrs)
			e.RawString("] ")
			e.CRLF()
		})
	}

	// Write UNSEEN if present
	if data.FirstUnseen > 0 {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.ResponseCode("UNSEEN", data.FirstUnseen)
			e.CRLF()
		})
	}

	// Write HIGHESTMODSEQ if present
	if data.HighestModSeq > 0 {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.ResponseCode("HIGHESTMODSEQ", data.HighestModSeq)
			e.CRLF()
		})
	}

	// Write MAILBOXID if present (RFC 8474)
	if data.MailboxID != "" {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("OK").SP()
			e.ResponseCode("MAILBOXID", "("+data.MailboxID+")")
			e.CRLF()
		})
	}

	// Update connection state
	ctx.Conn.SetMailbox(mailbox, data.ReadOnly)
	if err := ctx.Conn.SetState(imap.ConnStateSelected); err != nil {
		return err
	}

	// Tagged OK with READ-ONLY or READ-WRITE code
	code := "READ-WRITE"
	if data.ReadOnly {
		code = "READ-ONLY"
	}
	enc.Encode(func(e *wire.Encoder) {
		e.StatusResponse(ctx.Tag, "OK", code, "SELECT completed")
	})

	return nil
}
