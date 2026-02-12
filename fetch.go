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
	// PreviewLazy indicates the PREVIEW (LAZY) modifier was used (RFC 8970).
	PreviewLazy bool
	// SaveDate fetches the save date (RFC 8514).
	SaveDate bool
	// EmailID fetches the email ID (RFC 8474).
	EmailID bool
	// ThreadID fetches the thread ID (RFC 8474).
	ThreadID bool

	// BinarySection specifies BINARY[] and BINARY.PEEK[] sections to fetch (RFC 3516).
	BinarySection []*FetchItemBinarySection
	// BinarySizeSection specifies BINARY.SIZE[] sections to fetch (RFC 3516).
	// Each entry is a MIME part number list (e.g., []int{1, 2} for part "1.2").
	BinarySizeSection [][]int

	// ChangedSince only fetches messages with a mod-sequence greater than this value.
	ChangedSince uint64
	// Vanished requests VANISHED responses instead of EXPUNGE (QRESYNC).
	Vanished bool
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

// FetchItemBinarySection represents a BINARY[] or BINARY.PEEK[] fetch item (RFC 3516).
type FetchItemBinarySection struct {
	// Part is the MIME part number (e.g., []int{1, 2} for "1.2").
	Part []int
	// Peek prevents setting the \Seen flag (BINARY.PEEK).
	Peek bool
	// Partial is the partial byte range.
	Partial *SectionPartial
}

// BinarySizeData represents a BINARY.SIZE response item (RFC 3516).
type BinarySizeData struct {
	Part []int
	Size uint32
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
	Preview    string
	PreviewNIL bool
	SaveDate   *time.Time
	EmailID    string
	ThreadID   string

	// BodySection contains the fetched body sections.
	BodySection map[*FetchItemBodySection]SectionReader

	// BinarySection contains the fetched binary sections (RFC 3516).
	BinarySection map[*FetchItemBinarySection]SectionReader
	// BinarySizeSection contains the sizes for BINARY.SIZE requests (RFC 3516).
	BinarySizeSection []BinarySizeData
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
	Preview    string
	PreviewNIL bool
	SaveDate   *time.Time
	EmailID    string
	ThreadID   string

	// BodySection maps section names to their content.
	BodySection map[string][]byte

	// BinarySection maps part strings (e.g., "1.2") to decoded binary content.
	BinarySection map[string][]byte
	// BinarySizeSection maps part strings to decoded sizes.
	BinarySizeSection map[string]uint32
}
