package server

import (
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
	enc *ResponseEncoder
}

// NewFetchWriter creates a new FetchWriter.
func NewFetchWriter(enc *ResponseEncoder) *FetchWriter {
	return &FetchWriter{enc: enc}
}

// WriteFlags writes a FETCH FLAGS response.
func (w *FetchWriter) WriteFlags(seqNum uint32, flags []imap.Flag) {
	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Number(seqNum).SP().Atom("FETCH").SP().
			BeginList().Atom("FLAGS").SP().Flags(flagStrs).EndList().CRLF()
	})
}

// WriteFetchData writes a complete FETCH response for a message.
func (w *FetchWriter) WriteFetchData(data *imap.FetchMessageData) {
	w.enc.Encode(func(enc *wire.Encoder) {
		enc.Star().Number(data.SeqNum).SP().Atom("FETCH").SP().BeginList()

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

		if data.Preview != "" {
			sp()
			enc.Atom("PREVIEW").SP().String(data.Preview)
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
		enc.CRLF()
	})
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
	enc *ResponseEncoder
}

// NewExpungeWriter creates a new ExpungeWriter.
func NewExpungeWriter(enc *ResponseEncoder) *ExpungeWriter {
	return &ExpungeWriter{enc: enc}
}

// WriteExpunge writes an EXPUNGE response for a sequence number.
func (w *ExpungeWriter) WriteExpunge(seqNum uint32) {
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

// WriteExpunge writes an EXPUNGE response.
func (w *MoveWriter) WriteExpunge(seqNum uint32) {
	w.expunge.WriteExpunge(seqNum)
}

// WriteCopyData writes the OK response code with copy UID data.
func (w *MoveWriter) WriteCopyData(data *imap.CopyData) {
	// The copy data is written as part of the tagged OK response
	// This is handled by the dispatch layer
}
