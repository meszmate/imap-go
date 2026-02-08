package imap

// CopyData represents the result of a COPY or MOVE command.
type CopyData struct {
	// UIDValidity is the UID validity of the destination mailbox.
	UIDValidity uint32
	// SourceUIDs is the set of UIDs that were copied from the source.
	SourceUIDs UIDSet
	// DestUIDs is the set of UIDs in the destination mailbox.
	DestUIDs UIDSet
}
