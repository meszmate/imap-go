package memserver

import (
	"bufio"
	"bytes"
	"net/textproto"
	"strings"
	"time"

	imap "github.com/meszmate/imap-go"
)

// Message represents an in-memory email message.
type Message struct {
	UID          imap.UID
	Flags        []imap.Flag
	InternalDate time.Time
	Size         int64
	Body         []byte
}

// HasFlag returns true if the message has the given flag.
func (m *Message) HasFlag(flag imap.Flag) bool {
	for _, f := range m.Flags {
		if strings.EqualFold(string(f), string(flag)) {
			return true
		}
	}
	return false
}

// SetFlag adds a flag to the message if it doesn't already have it.
func (m *Message) SetFlag(flag imap.Flag) {
	if !m.HasFlag(flag) {
		m.Flags = append(m.Flags, flag)
	}
}

// RemoveFlag removes a flag from the message.
func (m *Message) RemoveFlag(flag imap.Flag) {
	for i, f := range m.Flags {
		if strings.EqualFold(string(f), string(flag)) {
			m.Flags = append(m.Flags[:i], m.Flags[i+1:]...)
			return
		}
	}
}

// CopyFlags returns a copy of the message's flags slice.
func (m *Message) CopyFlags() []imap.Flag {
	flags := make([]imap.Flag, len(m.Flags))
	copy(flags, m.Flags)
	return flags
}

// ParseEnvelope parses the message headers to build an Envelope.
func (m *Message) ParseEnvelope() *imap.Envelope {
	env := &imap.Envelope{}

	hdr := m.parseHeaders()
	if hdr == nil {
		return env
	}

	if dateStr := hdr.Get("Date"); dateStr != "" {
		// Try common date formats
		for _, layout := range []string{
			time.RFC1123Z,
			time.RFC1123,
			time.RFC822Z,
			time.RFC822,
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"2 Jan 2006 15:04:05 -0700",
		} {
			if t, err := time.Parse(layout, dateStr); err == nil {
				env.Date = t
				break
			}
		}
	}

	env.Subject = hdr.Get("Subject")
	env.From = parseAddressList(hdr.Get("From"))
	env.Sender = parseAddressList(hdr.Get("Sender"))
	env.ReplyTo = parseAddressList(hdr.Get("Reply-To"))
	env.To = parseAddressList(hdr.Get("To"))
	env.Cc = parseAddressList(hdr.Get("Cc"))
	env.Bcc = parseAddressList(hdr.Get("Bcc"))
	env.InReplyTo = hdr.Get("In-Reply-To")
	env.MessageID = hdr.Get("Message-ID")

	// If Sender is empty, use From
	if len(env.Sender) == 0 {
		env.Sender = env.From
	}
	// If Reply-To is empty, use From
	if len(env.ReplyTo) == 0 {
		env.ReplyTo = env.From
	}

	return env
}

// parseHeaders parses the message headers using textproto.
func (m *Message) parseHeaders() textproto.MIMEHeader {
	reader := bufio.NewReader(bytes.NewReader(m.Body))
	tp := textproto.NewReader(reader)
	hdr, err := tp.ReadMIMEHeader()
	if err != nil {
		// If there's an error, return what we have (partial headers are OK)
		return hdr
	}
	return hdr
}

// HeaderBytes returns the header portion of the message (up to the first blank line).
func (m *Message) HeaderBytes() []byte {
	idx := bytes.Index(m.Body, []byte("\r\n\r\n"))
	if idx < 0 {
		idx = bytes.Index(m.Body, []byte("\n\n"))
		if idx < 0 {
			return m.Body
		}
		return m.Body[:idx+2]
	}
	return m.Body[:idx+2]
}

// TextBytes returns the body portion of the message (after the first blank line).
func (m *Message) TextBytes() []byte {
	idx := bytes.Index(m.Body, []byte("\r\n\r\n"))
	if idx < 0 {
		idx = bytes.Index(m.Body, []byte("\n\n"))
		if idx < 0 {
			return nil
		}
		return m.Body[idx+2:]
	}
	return m.Body[idx+4:]
}

// parseAddressList parses a simple address list from a header value.
// This is a simplified parser that handles common formats:
//   - "user@host"
//   - "Name <user@host>"
//   - multiple addresses separated by commas
func parseAddressList(s string) []*imap.Address {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var addrs []*imap.Address
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		addr := parseAddress(part)
		if addr != nil {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// parseAddress parses a single email address.
func parseAddress(s string) *imap.Address {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	addr := &imap.Address{}

	// Check for "Name <user@host>" format
	if idx := strings.Index(s, "<"); idx >= 0 {
		addr.Name = strings.TrimSpace(s[:idx])
		// Remove surrounding quotes from name
		addr.Name = strings.Trim(addr.Name, "\"")
		end := strings.Index(s, ">")
		if end < 0 {
			end = len(s)
		}
		s = s[idx+1 : end]
	}

	// Parse user@host
	parts := strings.SplitN(s, "@", 2)
	addr.Mailbox = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		addr.Host = strings.TrimSpace(parts[1])
	}

	return addr
}
