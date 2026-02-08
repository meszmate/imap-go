package imap

import (
	"io"
	"time"
)

// FetchOptions specifies what message data items to fetch.
type FetchOptions struct {
	// BodySection specifies BODY[] sections to fetch.
	BodySection []*FetchItemBodySection
	// BodyStructure fetches the MIME body structure (BODYSTRUCTURE).
	BodyStructure bool
	// Envelope fetches the message envelope.
	Envelope bool
	// Flags fetches message flags.
	Flags bool
	// InternalDate fetches the internal date.
	InternalDate bool
	// RFC822Size fetches the RFC822 size.
	RFC822Size bool
	// UID fetches the UID.
	UID bool
	// ModSeq fetches the modification sequence (CONDSTORE).
	ModSeq bool
	// Preview fetches the message preview (RFC 8970).
	Preview bool
	// SaveDate fetches the save date (RFC 8514).
	SaveDate bool
	// EmailID fetches the email ID (RFC 8474).
	EmailID bool
	// ThreadID fetches the thread ID (RFC 8474).
	ThreadID bool

	// ChangedSince only fetches messages with a mod-sequence greater than this value.
	ChangedSince uint64
}

// FetchItemBodySection represents a BODY[section] fetch item.
type FetchItemBodySection struct {
	// Specifier is the section specifier (e.g., "HEADER", "TEXT", "HEADER.FIELDS").
	Specifier string
	// Part is the MIME part number (e.g., []int{1, 2} for "1.2").
	Part []int
	// Fields is the list of header fields for HEADER.FIELDS and HEADER.FIELDS.NOT.
	Fields []string
	// NotFields indicates this is HEADER.FIELDS.NOT.
	NotFields bool
	// Peek prevents setting the \Seen flag.
	Peek bool
	// Partial is the partial byte range.
	Partial *SectionPartial
}

// FetchMessageData represents the data returned for a single message in FETCH.
type FetchMessageData struct {
	// SeqNum is the message sequence number.
	SeqNum uint32

	// Items contains the fetched data items.
	Envelope      *Envelope
	BodyStructure *BodyStructure
	Flags         []Flag
	InternalDate  time.Time
	RFC822Size    int64
	UID           UID
	ModSeq        uint64
	Preview       string
	SaveDate      *time.Time
	EmailID       string
	ThreadID      string

	// BodySection contains the fetched body sections.
	BodySection map[*FetchItemBodySection]SectionReader
}

// SectionReader is a reader for a FETCH body section.
type SectionReader struct {
	io.Reader
	Size int64
}

// FetchMessageBuffer is a FetchMessageData that stores body sections in memory.
type FetchMessageBuffer struct {
	SeqNum        uint32
	Envelope      *Envelope
	BodyStructure *BodyStructure
	Flags         []Flag
	InternalDate  time.Time
	RFC822Size    int64
	UID           UID
	ModSeq        uint64
	Preview       string
	SaveDate      *time.Time
	EmailID       string
	ThreadID      string

	// BodySection maps section names to their content.
	BodySection map[string][]byte
}
