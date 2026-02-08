package imap

// StatusOptions specifies which mailbox status items to request.
type StatusOptions struct {
	NumMessages bool
	UIDNext     bool
	UIDValidity bool
	NumUnseen   bool
	NumRecent   bool // IMAP4rev1 only
	Size        bool // STATUS=SIZE (RFC 8438)
	AppendLimit bool // APPENDLIMIT (RFC 7889)
	NumDeleted  bool // for extensions
	HighestModSeq bool // CONDSTORE (RFC 7162)
	MailboxID   bool // OBJECTID (RFC 8474)
}

// StatusData represents the data returned by a STATUS command.
type StatusData struct {
	// Mailbox is the mailbox name.
	Mailbox string
	// NumMessages is the number of messages.
	NumMessages *uint32
	// UIDNext is the next UID.
	UIDNext *uint32
	// UIDValidity is the UID validity.
	UIDValidity *uint32
	// NumUnseen is the number of unseen messages.
	NumUnseen *uint32
	// NumRecent is the number of recent messages (IMAP4rev1 only).
	NumRecent *uint32
	// Size is the mailbox size in bytes (RFC 8438).
	Size *int64
	// AppendLimit is the maximum message size (RFC 7889).
	AppendLimit *uint32
	// NumDeleted is the number of deleted messages.
	NumDeleted *uint32
	// HighestModSeq is the highest modification sequence.
	HighestModSeq *uint64
	// MailboxID is the mailbox ID (RFC 8474).
	MailboxID string
}
