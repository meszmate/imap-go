package imap

// NamespaceData represents the result of a NAMESPACE command.
type NamespaceData struct {
	Personal []NamespaceDescriptor
	Other    []NamespaceDescriptor
	Shared   []NamespaceDescriptor
}

// NamespaceDescriptor describes a single namespace.
type NamespaceDescriptor struct {
	// Prefix is the namespace prefix.
	Prefix string
	// Delim is the hierarchy delimiter character (0 if none).
	Delim rune
}
