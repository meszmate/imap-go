package imap

// SelectOptions specifies options for the SELECT/EXAMINE command.
type SelectOptions struct {
	// ReadOnly opens the mailbox in read-only mode (EXAMINE).
	ReadOnly bool
	// CondStore enables CONDSTORE for this mailbox (RFC 7162).
	CondStore bool
	// QResync enables quick resync (RFC 7162).
	QResync *SelectQResync
}

// SelectQResync contains QRESYNC parameters.
type SelectQResync struct {
	UIDValidity uint32
	ModSeq      uint64
	KnownUIDs   *UIDSet
	SeqMatch    *QResyncSeqMatch
}

// QResyncSeqMatch contains known sequence number to UID mappings for QRESYNC.
type QResyncSeqMatch struct {
	SeqNums *SeqSet
	UIDs    *UIDSet
}

// SelectData represents the data returned by SELECT/EXAMINE.
type SelectData struct {
	// Flags is the list of defined flags in the mailbox.
	Flags []Flag
	// PermanentFlags is the list of flags that can be changed permanently.
	PermanentFlags []Flag
	// NumMessages is the number of messages in the mailbox.
	NumMessages uint32
	// NumRecent is the number of recent messages (IMAP4rev1 only).
	NumRecent uint32
	// UIDNext is the predicted next UID.
	UIDNext UID
	// UIDValidity is the UID validity value.
	UIDValidity uint32
	// FirstUnseen is the sequence number of the first unseen message.
	FirstUnseen uint32
	// HighestModSeq is the highest modification sequence (CONDSTORE).
	HighestModSeq uint64
	// ReadOnly is true if the mailbox was opened read-only.
	ReadOnly bool

	// MailboxID is the mailbox ID (RFC 8474).
	MailboxID string
}
