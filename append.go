package imap

import "time"

// AppendOptions specifies options for the APPEND command.
type AppendOptions struct {
	// Flags is the list of flags to set on the message.
	Flags []Flag
	// InternalDate is the internal date to set on the message.
	InternalDate time.Time
	// Binary indicates the message was sent using binary literal (~{N}) notation (RFC 3516).
	Binary bool
	// UTF8 indicates the message was sent using UTF8 literal notation (RFC 6855).
	UTF8 bool
}

// AppendData represents the result of an APPEND command.
type AppendData struct {
	// UIDValidity is the UID validity of the destination mailbox.
	UIDValidity uint32
	// UID is the UID assigned to the appended message (UIDPLUS).
	UID UID
}
