// Package imap implements the IMAP protocol (RFC 9051, RFC 3501).
//
// This package provides shared types used by both the client and server
// implementations. It supports IMAP4rev1 (RFC 3501) and IMAP4rev2 (RFC 9051).
package imap

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ConnState represents the state of an IMAP connection.
type ConnState int

const (
	// ConnStateNotAuthenticated is the state before authentication.
	ConnStateNotAuthenticated ConnState = iota
	// ConnStateAuthenticated is the state after successful authentication.
	ConnStateAuthenticated
	// ConnStateSelected is the state after a mailbox has been selected.
	ConnStateSelected
	// ConnStateLogout is the state after the LOGOUT command.
	ConnStateLogout
)

// String returns the string representation of the connection state.
func (s ConnState) String() string {
	switch s {
	case ConnStateNotAuthenticated:
		return "not authenticated"
	case ConnStateAuthenticated:
		return "authenticated"
	case ConnStateSelected:
		return "selected"
	case ConnStateLogout:
		return "logout"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Flag represents an IMAP message flag.
type Flag string

// Standard message flags defined in RFC 9051.
const (
	FlagSeen     Flag = "\\Seen"
	FlagAnswered Flag = "\\Answered"
	FlagFlagged  Flag = "\\Flagged"
	FlagDeleted  Flag = "\\Deleted"
	FlagDraft    Flag = "\\Draft"
	FlagRecent   Flag = "\\Recent" // IMAP4rev1 only, removed in rev2
	FlagWildcard Flag = "\\*"      // Permanent flags wildcard
)

// MailboxAttr represents a mailbox attribute.
type MailboxAttr string

// Standard mailbox attributes.
const (
	MailboxAttrNoInferiors   MailboxAttr = "\\Noinferiors"
	MailboxAttrNoSelect      MailboxAttr = "\\Noselect"
	MailboxAttrMarked        MailboxAttr = "\\Marked"
	MailboxAttrUnmarked      MailboxAttr = "\\Unmarked"
	MailboxAttrHasChildren   MailboxAttr = "\\HasChildren"
	MailboxAttrHasNoChildren MailboxAttr = "\\HasNoChildren"
	MailboxAttrNonExistent   MailboxAttr = "\\NonExistent"
	MailboxAttrSubscribed    MailboxAttr = "\\Subscribed"
	MailboxAttrRemote        MailboxAttr = "\\Remote"

	// Special-use attributes (RFC 6154)
	MailboxAttrAll     MailboxAttr = "\\All"
	MailboxAttrArchive MailboxAttr = "\\Archive"
	MailboxAttrDrafts  MailboxAttr = "\\Drafts"
	MailboxAttrFlagged MailboxAttr = "\\Flagged"
	MailboxAttrJunk    MailboxAttr = "\\Junk"
	MailboxAttrSent    MailboxAttr = "\\Sent"
	MailboxAttrTrash   MailboxAttr = "\\Trash"
)

// LiteralReader reads a literal string from an IMAP connection.
type LiteralReader struct {
	io.Reader
	Size int64
}

// NumKind indicates whether a number set uses sequence numbers or UIDs.
type NumKind int

const (
	// NumKindSeq indicates sequence numbers.
	NumKindSeq NumKind = iota
	// NumKindUID indicates unique identifiers.
	NumKindUID
)

// String returns the string representation of the number kind.
func (k NumKind) String() string {
	switch k {
	case NumKindSeq:
		return "seq"
	case NumKindUID:
		return "uid"
	default:
		return fmt.Sprintf("unknown(%d)", int(k))
	}
}

// BodySectionName represents a BODY section specification.
type BodySectionName struct {
	// Specifier is the section specifier (HEADER, HEADER.FIELDS, TEXT, MIME, or empty).
	Specifier string
	// Part is the MIME part number (e.g., []int{1, 2} for "1.2").
	Part []int
	// Fields is the list of header fields for HEADER.FIELDS and HEADER.FIELDS.NOT.
	Fields []string
	// NotFields indicates whether Fields is a NOT list.
	NotFields bool
	// Peek indicates whether the \Seen flag should not be set (BODY.PEEK).
	Peek bool
	// Partial is the partial byte range (<offset.count>).
	Partial *SectionPartial
}

// SectionPartial represents a partial byte range.
type SectionPartial struct {
	Offset int64
	Count  int64
}

// Address represents an email address in an envelope.
type Address struct {
	Name    string
	Mailbox string
	Host    string
}

// String returns the email address in "Name <mailbox@host>" format.
func (a *Address) String() string {
	addr := a.Mailbox + "@" + a.Host
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, addr)
	}
	return addr
}

// Envelope represents the envelope structure of a message (RFC 2822 header fields).
type Envelope struct {
	Date      time.Time
	Subject   string
	From      []*Address
	Sender    []*Address
	ReplyTo   []*Address
	To        []*Address
	Cc        []*Address
	Bcc       []*Address
	InReplyTo string
	MessageID string
}

// BodyStructure represents the MIME structure of a message.
type BodyStructure struct {
	// Type is the MIME type (e.g., "text", "multipart").
	Type string
	// Subtype is the MIME subtype (e.g., "plain", "mixed").
	Subtype string
	// Params are the Content-Type parameters (e.g., charset).
	Params map[string]string
	// ID is the Content-ID.
	ID string
	// Description is the Content-Description.
	Description string
	// Encoding is the Content-Transfer-Encoding.
	Encoding string
	// Size is the body size in bytes.
	Size uint32
	// Envelope is the envelope of an embedded message/rfc822 part.
	Envelope *Envelope
	// BodyStructure is the body structure of an embedded message/rfc822 part.
	BodyStructure *BodyStructure
	// Lines is the number of text lines (for text/* and message/rfc822).
	Lines uint32

	// Extended fields (only in BODYSTRUCTURE, not BODY)
	MD5         string
	Disposition string
	DispositionParams map[string]string
	Language    []string
	Location    string

	// For multipart bodies
	Children []BodyStructure
}

// IsMultipart returns true if this body structure is multipart.
func (bs *BodyStructure) IsMultipart() bool {
	return strings.EqualFold(bs.Type, "multipart")
}

// InternalDate represents an IMAP internal date.
type InternalDate time.Time

// InternalDateLayout is the format used for IMAP internal dates.
const InternalDateLayout = "02-Jan-2006 15:04:05 -0700"

// String returns the date in IMAP format.
func (d InternalDate) String() string {
	return time.Time(d).Format(InternalDateLayout)
}

// CreateOptions contains options for the CREATE command.
type CreateOptions struct {
	// SpecialUse is the special-use attribute for the mailbox (RFC 6154).
	SpecialUse MailboxAttr
}
