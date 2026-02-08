package memserver

import "sync"

// UserData holds all mailbox data for a single user.
type UserData struct {
	mu        sync.RWMutex
	Mailboxes map[string]*Mailbox
}

// NewUserData creates a new UserData with a default INBOX.
func NewUserData() *UserData {
	inbox := NewMailbox("INBOX")
	inbox.Subscribed = true

	return &UserData{
		Mailboxes: map[string]*Mailbox{
			"INBOX": inbox,
		},
	}
}

// GetMailbox returns the mailbox with the given name.
// INBOX is matched case-insensitively per the IMAP spec.
func (u *UserData) GetMailbox(name string) *Mailbox {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.getMailboxLocked(name)
}

// getMailboxLocked returns a mailbox without locking. Caller must hold at least a read lock.
func (u *UserData) getMailboxLocked(name string) *Mailbox {
	// INBOX is case-insensitive
	mbox, ok := u.Mailboxes[name]
	if ok {
		return mbox
	}
	// Try case-insensitive match for INBOX
	if normalizeINBOX(name) == "INBOX" {
		return u.Mailboxes["INBOX"]
	}
	return nil
}

// CreateMailbox creates a new mailbox with the given name.
func (u *UserData) CreateMailbox(name string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.getMailboxLocked(name) != nil {
		return ErrMailboxAlreadyExists
	}

	mbox := NewMailbox(name)
	u.Mailboxes[name] = mbox
	return nil
}

// DeleteMailbox deletes the mailbox with the given name.
func (u *UserData) DeleteMailbox(name string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if normalizeINBOX(name) == "INBOX" {
		return &IMAPError{Message: "cannot delete INBOX"}
	}

	if u.getMailboxLocked(name) == nil {
		return ErrNoSuchMailbox
	}

	delete(u.Mailboxes, name)
	return nil
}

// RenameMailbox renames a mailbox.
func (u *UserData) RenameMailbox(oldName, newName string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	mbox := u.getMailboxLocked(oldName)
	if mbox == nil {
		return ErrNoSuchMailbox
	}

	if u.getMailboxLocked(newName) != nil {
		return ErrMailboxAlreadyExists
	}

	// Remove old entry and add new one
	delete(u.Mailboxes, oldName)
	mbox.Name = newName
	u.Mailboxes[newName] = mbox

	return nil
}

// MailboxNames returns a list of all mailbox names.
func (u *UserData) MailboxNames() []string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	names := make([]string, 0, len(u.Mailboxes))
	for name := range u.Mailboxes {
		names = append(names, name)
	}
	return names
}

// normalizeINBOX normalizes a mailbox name to "INBOX" if it matches case-insensitively.
func normalizeINBOX(name string) string {
	if len(name) == 5 {
		upper := ""
		for _, c := range name {
			if c >= 'a' && c <= 'z' {
				upper += string(c - 32)
			} else {
				upper += string(c)
			}
		}
		if upper == "INBOX" {
			return "INBOX"
		}
	}
	return name
}

// IMAPError is a simple error type for IMAP errors.
type IMAPError struct {
	Message string
}

func (e *IMAPError) Error() string {
	return e.Message
}
