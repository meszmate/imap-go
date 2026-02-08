package server

import (
	imap "github.com/meszmate/imap-go"
)

// Session is the interface that server backends must implement.
// Each connection creates a new Session via the Server's NewSession callback.
type Session interface {
	// Close is called when the connection is closed.
	Close() error

	// Login authenticates the user with a username and password.
	Login(username, password string) error

	// Select opens a mailbox.
	Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error)

	// Create creates a new mailbox.
	Create(mailbox string, options *imap.CreateOptions) error

	// Delete deletes a mailbox.
	Delete(mailbox string) error

	// Rename renames a mailbox.
	Rename(mailbox, newName string) error

	// Subscribe subscribes to a mailbox.
	Subscribe(mailbox string) error

	// Unsubscribe unsubscribes from a mailbox.
	Unsubscribe(mailbox string) error

	// List lists mailboxes matching the given patterns.
	List(w *ListWriter, ref string, patterns []string, options *imap.ListOptions) error

	// Status returns the status of a mailbox.
	Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error)

	// Append appends a message to a mailbox.
	Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error)

	// Poll checks for mailbox updates without blocking.
	Poll(w *UpdateWriter, allowExpunge bool) error

	// Idle waits for mailbox updates until stop is closed.
	Idle(w *UpdateWriter, stop <-chan struct{}) error

	// Unselect closes the current mailbox without expunging.
	Unselect() error

	// Expunge permanently removes messages marked as deleted.
	Expunge(w *ExpungeWriter, uids *imap.UIDSet) error

	// Search searches for messages matching the criteria.
	Search(kind NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)

	// Fetch retrieves message data.
	Fetch(w *FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error

	// Store modifies message flags.
	Store(w *FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error

	// Copy copies messages to another mailbox.
	Copy(numSet imap.NumSet, dest string) (*imap.CopyData, error)
}

// SessionMove is an optional interface for sessions that support the MOVE command.
type SessionMove interface {
	Move(w *MoveWriter, numSet imap.NumSet, dest string) error
}

// SessionNamespace is an optional interface for sessions that support NAMESPACE.
type SessionNamespace interface {
	Namespace() (*imap.NamespaceData, error)
}

// SessionID is an optional interface for sessions that support the ID command.
type SessionID interface {
	ID(clientID imap.IDData) (*imap.IDData, error)
}

// SessionSort is an optional interface for sessions that support SORT.
type SessionSort interface {
	Sort(kind NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SortData, error)
}

// SessionThread is an optional interface for sessions that support THREAD.
type SessionThread interface {
	Thread(kind NumKind, algorithm imap.ThreadAlgorithm, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.ThreadData, error)
}
