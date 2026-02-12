package server

import (
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/wire"
)

// ResponseEncoder wraps a wire.Encoder with thread-safe access.
type ResponseEncoder struct {
	mu  sync.Mutex
	enc *wire.Encoder
}

// NewResponseEncoder creates a new ResponseEncoder.
func NewResponseEncoder(enc *wire.Encoder) *ResponseEncoder {
	return &ResponseEncoder{enc: enc}
}

// Encode calls the given function with exclusive access to the encoder.
func (re *ResponseEncoder) Encode(fn func(enc *wire.Encoder)) {
	re.mu.Lock()
	defer re.mu.Unlock()
	fn(re.enc)
	_ = re.enc.Flush()
}

// FetchWriter writes FETCH response data.
type FetchWriter struct {
	enc     *ResponseEncoder
	uidOnly bool
}

// NewFetchWriter creates a new FetchWriter.
func NewFetchWriter(enc *ResponseEncoder) *FetchWriter {
	return &FetchWriter{enc: enc}
}

// SetUIDOnly enables UIDONLY mode where responses use UIDFETCH with UIDs
// instead of FETCH with sequence numbers (RFC 9586).
func (w *FetchWriter) SetUIDOnly(enabled bool) {
	w.uidOnly = enabled
}

// WriteFlags writes a FETCH FLAGS response.
// In UIDONLY mode, seqNum is treated as a UID and UIDFETCH is used.
func (w *FetchWriter) WriteFlags(seqNum uint32, flags []imap.Flag) {
	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	keyword := "FETCH"
	if w.uidOnly {
		keyword = "UIDFETCH"
	}
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Number(seqNum).SP().Atom(keyword).SP().
			BeginList().Atom("FLAGS").SP().Flags(flagStrs).EndList().CRLF()
	})
}

// WriteFetchData writes a complete FETCH response for a message.
// In UIDONLY mode, uses the UID as the message number and UIDFETCH as the keyword.
func (w *FetchWriter) WriteFetchData(data *imap.FetchMessageData) {
	w.enc.Encode(func(enc *wire.Encoder) {
		num := data.SeqNum
		keyword := "FETCH"
		if w.uidOnly {
			num = uint32(data.UID)
			keyword = "UIDFETCH"
		}
		enc.Star().Number(num).SP().Atom(keyword).SP().BeginList()

		first := true
		sp := func() {
			if !first {
				enc.SP()
			}
			first = false
		}

		if data.Flags != nil {
			sp()
			flagStrs := make([]string, len(data.Flags))
			for i, f := range data.Flags {
				flagStrs[i] = string(f)
			}
			enc.Atom("FLAGS").SP().Flags(flagStrs)
		}

		if data.UID != 0 {
			sp()
			enc.Atom("UID").SP().Number(uint32(data.UID))
		}

		if data.RFC822Size != 0 {
			sp()
			enc.Atom("RFC822.SIZE").SP().Number64(uint64(data.RFC822Size))
		}

		if !data.InternalDate.IsZero() {
			sp()
			enc.Atom("INTERNALDATE").SP().DateTime(data.InternalDate)
		}

		if data.Envelope != nil {
			sp()
			enc.Atom("ENVELOPE").SP()
			writeEnvelope(enc, data.Envelope)
		}

		if data.ModSeq != 0 {
			sp()
			enc.Atom("MODSEQ").SP().BeginList().Number64(data.ModSeq).EndList()
		}

		if data.EmailID != "" {
			sp()
			enc.Atom("EMAILID").SP().BeginList().AString(data.EmailID).EndList()
		}

		if data.ThreadID != "" {
			sp()
			enc.Atom("THREADID").SP().BeginList().AString(data.ThreadID).EndList()
		}

		if data.SaveDate != nil {
			sp()
			enc.Atom("SAVEDATE").SP().DateTime(*data.SaveDate)
		} else if data.SaveDateNIL {
			sp()
			enc.Atom("SAVEDATE").SP().Nil()
		}

		if data.Preview != "" {
			sp()
			enc.Atom("PREVIEW").SP().String(data.Preview)
		} else if data.PreviewNIL {
			sp()
			enc.Atom("PREVIEW").SP().Nil()
		}

		// Write BINARY sections (RFC 3516)
		for section, reader := range data.BinarySection {
			sp()
			enc.Atom("BINARY[" + formatPart(section.Part) + "]").SP()
			binaryData, _ := io.ReadAll(reader.Reader)
			enc.BinaryLiteral(binaryData)
		}

		// Write BINARY.SIZE sections (RFC 3516)
		for _, bs := range data.BinarySizeSection {
			sp()
			enc.Atom("BINARY.SIZE[" + formatPart(bs.Part) + "]").SP().Number(bs.Size)
		}

		enc.EndList().CRLF()
	})
}

func writeEnvelope(enc *wire.Encoder, env *imap.Envelope) {
	enc.BeginList()
	if env.Date.IsZero() {
		enc.Nil()
	} else {
		enc.QuotedString(env.Date.Format(time.RFC822Z))
	}
	enc.SP()
	if env.Subject == "" {
		enc.Nil()
	} else {
		enc.String(env.Subject)
	}
	enc.SP()
	writeAddressList(enc, env.From)
	enc.SP()
	writeAddressList(enc, env.Sender)
	enc.SP()
	writeAddressList(enc, env.ReplyTo)
	enc.SP()
	writeAddressList(enc, env.To)
	enc.SP()
	writeAddressList(enc, env.Cc)
	enc.SP()
	writeAddressList(enc, env.Bcc)
	enc.SP()
	if env.InReplyTo == "" {
		enc.Nil()
	} else {
		enc.String(env.InReplyTo)
	}
	enc.SP()
	if env.MessageID == "" {
		enc.Nil()
	} else {
		enc.String(env.MessageID)
	}
	enc.EndList()
}

func writeAddressList(enc *wire.Encoder, addrs []*imap.Address) {
	if len(addrs) == 0 {
		enc.Nil()
		return
	}
	enc.BeginList()
	for i, addr := range addrs {
		if i > 0 {
			enc.SP()
		}
		enc.BeginList()
		if addr.Name != "" {
			enc.String(addr.Name)
		} else {
			enc.Nil()
		}
		enc.SP().Nil() // at-domain-list (always NIL in modern usage)
		enc.SP()
		if addr.Mailbox != "" {
			enc.String(addr.Mailbox)
		} else {
			enc.Nil()
		}
		enc.SP()
		if addr.Host != "" {
			enc.String(addr.Host)
		} else {
			enc.Nil()
		}
		enc.EndList()
	}
	enc.EndList()
}

// ListWriter writes LIST responses.
type ListWriter struct {
	enc *ResponseEncoder
}

// NewListWriter creates a new ListWriter.
func NewListWriter(enc *ResponseEncoder) *ListWriter {
	return &ListWriter{enc: enc}
}

// WriteList writes a single LIST response.
func (w *ListWriter) WriteList(data *imap.ListData) {
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("LIST").SP()

		// Attributes
		enc.BeginList()
		for i, attr := range data.Attrs {
			if i > 0 {
				enc.SP()
			}
			enc.Atom(string(attr))
		}
		enc.EndList()

		// Delimiter
		enc.SP()
		if data.Delim == 0 {
			enc.Nil()
		} else {
			enc.QuotedString(string(data.Delim))
		}

		// Mailbox name
		enc.SP().MailboxName(data.Mailbox)

		// Extended data items (RFC 5258)
		if hasExtendedData(data) {
			enc.SP().BeginList()
			first := true
			sp := func() {
				if !first {
					enc.SP()
				}
				first = false
			}
			if len(data.ChildInfo) > 0 {
				sp()
				enc.QuotedString("CHILDINFO").SP().BeginList()
				for i, ci := range data.ChildInfo {
					if i > 0 {
						enc.SP()
					}
					enc.QuotedString(ci)
				}
				enc.EndList()
			}
			if data.OldName != "" {
				sp()
				enc.QuotedString("OLDNAME").SP().BeginList().MailboxName(data.OldName).EndList()
			}
			if data.MyRights != "" {
				sp()
				enc.QuotedString("MYRIGHTS").SP().QuotedString(data.MyRights)
			}
			if data.Metadata != nil {
				sp()
				enc.QuotedString("METADATA").SP().BeginList()
				mFirst := true
				for k, v := range data.Metadata {
					if !mFirst {
						enc.SP()
					}
					enc.QuotedString(k).SP().QuotedString(v)
					mFirst = false
				}
				enc.EndList()
			}
			enc.EndList()
		}

		enc.CRLF()
	})

	// STATUS is emitted as a separate untagged response
	if data.Status != nil {
		w.enc.Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("STATUS").SP().MailboxName(data.Mailbox).SP().BeginList()
			first := true
			sp := func() {
				if !first {
					enc.SP()
				}
				first = false
			}
			if data.Status.NumMessages != nil {
				sp()
				enc.Atom("MESSAGES").SP().Number(*data.Status.NumMessages)
			}
			if data.Status.UIDNext != nil {
				sp()
				enc.Atom("UIDNEXT").SP().Number(*data.Status.UIDNext)
			}
			if data.Status.UIDValidity != nil {
				sp()
				enc.Atom("UIDVALIDITY").SP().Number(*data.Status.UIDValidity)
			}
			if data.Status.NumUnseen != nil {
				sp()
				enc.Atom("UNSEEN").SP().Number(*data.Status.NumUnseen)
			}
			if data.Status.NumRecent != nil {
				sp()
				enc.Atom("RECENT").SP().Number(*data.Status.NumRecent)
			}
			if data.Status.Size != nil {
				sp()
				enc.Atom("SIZE").SP().Number64(uint64(*data.Status.Size))
			}
			if data.Status.HighestModSeq != nil {
				sp()
				enc.Atom("HIGHESTMODSEQ").SP().Number64(*data.Status.HighestModSeq)
			}
			enc.EndList().CRLF()
		})
	}
}

// formatPart formats a MIME part number list (e.g., []int{1, 2}) as "1.2".
func formatPart(part []int) string {
	if len(part) == 0 {
		return ""
	}
	s := make([]string, len(part))
	for i, p := range part {
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ".")
}

// hasExtendedData returns true if any extended data fields are set in ListData.
func hasExtendedData(data *imap.ListData) bool {
	return len(data.ChildInfo) > 0 || data.OldName != "" || data.MyRights != "" || data.Metadata != nil
}

// UpdateWriter writes unsolicited updates.
type UpdateWriter struct {
	enc *ResponseEncoder
}

// NewUpdateWriter creates a new UpdateWriter.
func NewUpdateWriter(enc *ResponseEncoder) *UpdateWriter {
	return &UpdateWriter{enc: enc}
}

// WriteExists writes an EXISTS update.
func (w *UpdateWriter) WriteExists(num uint32) {
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.NumResponse(num, "EXISTS")
	})
}

// WriteExpunge writes an EXPUNGE update.
func (w *UpdateWriter) WriteExpunge(seqNum uint32) {
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.NumResponse(seqNum, "EXPUNGE")
	})
}

// WriteRecent writes a RECENT update.
func (w *UpdateWriter) WriteRecent(num uint32) {
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.NumResponse(num, "RECENT")
	})
}

// WriteFlags writes a FLAGS update (mailbox flags).
func (w *UpdateWriter) WriteFlags(flags []imap.Flag) {
	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("FLAGS").SP().Flags(flagStrs).CRLF()
	})
}

// WriteMessageFlags writes updated flags for a message.
func (w *UpdateWriter) WriteMessageFlags(seqNum uint32, flags []imap.Flag) {
	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Number(seqNum).SP().Atom("FETCH").SP().
			BeginList().Atom("FLAGS").SP().Flags(flagStrs).EndList().CRLF()
	})
}

// ExpungeWriter writes EXPUNGE responses.
type ExpungeWriter struct {
	enc     *ResponseEncoder
	uidOnly bool
}

// NewExpungeWriter creates a new ExpungeWriter.
func NewExpungeWriter(enc *ResponseEncoder) *ExpungeWriter {
	return &ExpungeWriter{enc: enc}
}

// SetUIDOnly enables UIDONLY mode where VANISHED responses are emitted
// instead of EXPUNGE (RFC 9586). When enabled, the num parameter to
// WriteExpunge is treated as a UID.
func (w *ExpungeWriter) SetUIDOnly(enabled bool) {
	w.uidOnly = enabled
}

// WriteExpunge writes an EXPUNGE response for a sequence number.
// In UIDONLY mode, emits * VANISHED <uid> instead.
func (w *ExpungeWriter) WriteExpunge(seqNum uint32) {
	if w.uidOnly {
		w.enc.Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("VANISHED").SP().Atom(strconv.FormatUint(uint64(seqNum), 10)).CRLF()
		})
		return
	}
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.NumResponse(seqNum, "EXPUNGE")
	})
}

// MoveWriter writes MOVE response data (combines expunge + copy data).
type MoveWriter struct {
	expunge *ExpungeWriter
	enc     *ResponseEncoder
}

// NewMoveWriter creates a new MoveWriter.
func NewMoveWriter(enc *ResponseEncoder) *MoveWriter {
	return &MoveWriter{
		expunge: NewExpungeWriter(enc),
		enc:     enc,
	}
}

// SetUIDOnly enables UIDONLY mode on the MoveWriter's expunge output,
// emitting VANISHED instead of EXPUNGE (RFC 9586).
func (w *MoveWriter) SetUIDOnly(enabled bool) {
	w.expunge.SetUIDOnly(enabled)
}

// WriteExpunge writes an EXPUNGE response.
func (w *MoveWriter) WriteExpunge(seqNum uint32) {
	w.expunge.WriteExpunge(seqNum)
}

// WriteCopyData writes the OK response code with copy UID data.
func (w *MoveWriter) WriteCopyData(data *imap.CopyData) {
	// The copy data is written as part of the tagged OK response
	// This is handled by the dispatch layer
}
