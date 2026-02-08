package imap

// ThreadAlgorithm represents a threading algorithm.
type ThreadAlgorithm string

const (
	ThreadAlgorithmOrderedSubject ThreadAlgorithm = "ORDEREDSUBJECT"
	ThreadAlgorithmReferences     ThreadAlgorithm = "REFERENCES"
)

// Thread represents a single thread in the response.
type Thread struct {
	// Num is the message sequence number or UID at this node.
	Num uint32
	// Children are sub-threads branching from this message.
	Children []Thread
}

// ThreadData represents the result of a THREAD command.
type ThreadData struct {
	Threads []Thread
}
