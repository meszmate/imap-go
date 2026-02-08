package imap

// ListOptions specifies options for the LIST command.
type ListOptions struct {
	// SelectSubscribed only returns subscribed mailboxes.
	SelectSubscribed bool
	// SelectRemote includes remote mailboxes.
	SelectRemote bool
	// SelectRecursiveMatch returns all children.
	SelectRecursiveMatch bool
	// SelectSpecialUse only returns mailboxes with special-use attributes.
	SelectSpecialUse bool

	// ReturnSubscribed includes SUBSCRIBED return option.
	ReturnSubscribed bool
	// ReturnChildren includes CHILDREN return option.
	ReturnChildren bool
	// ReturnSpecialUse includes SPECIAL-USE return option.
	ReturnSpecialUse bool
	// ReturnStatus includes STATUS return option.
	ReturnStatus *StatusOptions
	// ReturnMyRights includes MYRIGHTS return option (RFC 8440).
	ReturnMyRights bool
	// ReturnMetadata includes METADATA return option (RFC 9590).
	ReturnMetadata *ListReturnMetadata
}

// ListReturnMetadata specifies metadata to return with LIST.
type ListReturnMetadata struct {
	// Options contains the metadata options.
	Options []string
	// MaxSize is the maximum size of metadata to return.
	MaxSize int64
	// Depth specifies how deep to look for metadata values.
	Depth string // "0", "1", "infinity"
}

// ListData represents a single LIST response.
type ListData struct {
	// Attrs is the list of mailbox attributes.
	Attrs []MailboxAttr
	// Delim is the hierarchy delimiter character (0 if none).
	Delim rune
	// Mailbox is the mailbox name.
	Mailbox string

	// Extended data
	// OldName is set during RENAME notifications.
	OldName string
	// ChildInfo contains child info extended data.
	ChildInfo []string
	// Status is included when LIST-STATUS is requested.
	Status *StatusData
	// MyRights is included when LIST-MYRIGHTS is requested.
	MyRights string
	// Metadata is included when LIST-METADATA is requested.
	Metadata map[string]string
}
