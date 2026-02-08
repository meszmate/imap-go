package imap

// MetadataEntry represents a metadata entry.
type MetadataEntry struct {
	// Name is the metadata entry name.
	Name string
	// Value is the metadata value. Nil means the entry should be removed.
	Value *string
}

// MetadataOptions specifies options for GETMETADATA.
type MetadataOptions struct {
	// MaxSize limits the size of returned values.
	MaxSize *int64
	// Depth limits the depth of returned entries.
	Depth string // "0", "1", "infinity"
}

// MetadataData represents the result of a GETMETADATA command.
type MetadataData struct {
	// Mailbox is the mailbox name (empty for server-level).
	Mailbox string
	// Entries maps entry names to values.
	Entries map[string]*string
}
