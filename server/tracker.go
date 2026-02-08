package server

import (
	"sync"

	imap "github.com/meszmate/imap-go"
)

// MailboxTracker tracks the state of a selected mailbox.
type MailboxTracker struct {
	mu          sync.RWMutex
	name        string
	numMessages uint32
	uidNext     imap.UID
	uidValidity uint32
	sessions    map[*SessionTracker]struct{}
}

// NewMailboxTracker creates a new tracker for a mailbox.
func NewMailboxTracker(name string, numMessages uint32, uidValidity uint32, uidNext imap.UID) *MailboxTracker {
	return &MailboxTracker{
		name:        name,
		numMessages: numMessages,
		uidNext:     uidNext,
		uidValidity: uidValidity,
		sessions:    make(map[*SessionTracker]struct{}),
	}
}

// Name returns the mailbox name.
func (t *MailboxTracker) Name() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

// NumMessages returns the current message count.
func (t *MailboxTracker) NumMessages() uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.numMessages
}

// QueueUpdate queues an update for all sessions watching this mailbox.
func (t *MailboxTracker) QueueUpdate(update Update) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for st := range t.sessions {
		st.queueUpdate(update)
	}
}

// QueueExpunge queues an expunge notification.
func (t *MailboxTracker) QueueExpunge(seqNum uint32) {
	t.mu.Lock()
	if t.numMessages > 0 {
		t.numMessages--
	}
	t.mu.Unlock()
	t.QueueUpdate(ExpungeUpdate{SeqNum: seqNum})
}

// QueueNewMessage notifies sessions of a new message.
func (t *MailboxTracker) QueueNewMessage() {
	t.mu.Lock()
	t.numMessages++
	num := t.numMessages
	t.mu.Unlock()
	t.QueueUpdate(ExistsUpdate{NumMessages: num})
}

// QueueFlagsUpdate notifies sessions of a flag change.
func (t *MailboxTracker) QueueFlagsUpdate(seqNum uint32, flags []imap.Flag) {
	t.QueueUpdate(FetchFlagsUpdate{SeqNum: seqNum, Flags: flags})
}

func (t *MailboxTracker) addSession(st *SessionTracker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessions[st] = struct{}{}
}

func (t *MailboxTracker) removeSession(st *SessionTracker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessions, st)
}

// SessionTracker tracks pending updates for a single session.
type SessionTracker struct {
	mu      sync.Mutex
	mailbox *MailboxTracker
	updates []Update
}

// NewSessionTracker creates a new session tracker.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{}
}

// Select associates the session with a mailbox.
func (st *SessionTracker) Select(mbox *MailboxTracker) {
	st.mu.Lock()
	if st.mailbox != nil {
		st.mailbox.removeSession(st)
	}
	st.mailbox = mbox
	st.updates = nil
	st.mu.Unlock()
	if mbox != nil {
		mbox.addSession(st)
	}
}

// Unselect disassociates the session from the current mailbox.
func (st *SessionTracker) Unselect() {
	st.mu.Lock()
	if st.mailbox != nil {
		st.mailbox.removeSession(st)
	}
	st.mailbox = nil
	st.updates = nil
	st.mu.Unlock()
}

// Flush sends all pending updates to the writer and clears them.
func (st *SessionTracker) Flush(w *UpdateWriter, allowExpunge bool) {
	st.mu.Lock()
	updates := st.updates
	st.updates = nil
	st.mu.Unlock()

	for _, u := range updates {
		switch u := u.(type) {
		case ExistsUpdate:
			w.WriteExists(u.NumMessages)
		case ExpungeUpdate:
			if allowExpunge {
				w.WriteExpunge(u.SeqNum)
			}
		case FetchFlagsUpdate:
			w.WriteMessageFlags(u.SeqNum, u.Flags)
		}
	}
}

func (st *SessionTracker) queueUpdate(update Update) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.updates = append(st.updates, update)
}

// Update is an interface for mailbox updates.
type Update interface {
	updateType() string
}

// ExistsUpdate indicates the mailbox message count changed.
type ExistsUpdate struct {
	NumMessages uint32
}

func (ExistsUpdate) updateType() string { return "EXISTS" }

// ExpungeUpdate indicates a message was expunged.
type ExpungeUpdate struct {
	SeqNum uint32
}

func (ExpungeUpdate) updateType() string { return "EXPUNGE" }

// FetchFlagsUpdate indicates message flags changed.
type FetchFlagsUpdate struct {
	SeqNum uint32
	Flags  []imap.Flag
}

func (FetchFlagsUpdate) updateType() string { return "FETCH" }
