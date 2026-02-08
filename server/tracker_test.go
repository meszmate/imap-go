package server

import (
	"bytes"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/wire"
)

// --- MailboxTracker tests ---

func TestNewMailboxTracker(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 10, 42, 100)

	if mt.Name() != "INBOX" {
		t.Fatalf("Name: expected %q, got %q", "INBOX", mt.Name())
	}
	if mt.NumMessages() != 10 {
		t.Fatalf("NumMessages: expected %d, got %d", 10, mt.NumMessages())
	}
	if mt.uidValidity != 42 {
		t.Fatalf("uidValidity: expected %d, got %d", 42, mt.uidValidity)
	}
	if mt.uidNext != 100 {
		t.Fatalf("uidNext: expected %d, got %d", 100, mt.uidNext)
	}
	if mt.sessions == nil {
		t.Fatal("sessions map is nil")
	}
}

func TestMailboxTracker_QueueNewMessage(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	mt.QueueNewMessage()

	if mt.NumMessages() != 6 {
		t.Fatalf("NumMessages: expected %d, got %d", 6, mt.NumMessages())
	}

	// Check update was queued
	st.mu.Lock()
	if len(st.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.updates))
	}
	update, ok := st.updates[0].(ExistsUpdate)
	if !ok {
		t.Fatalf("expected ExistsUpdate, got %T", st.updates[0])
	}
	if update.NumMessages != 6 {
		t.Fatalf("ExistsUpdate.NumMessages: expected %d, got %d", 6, update.NumMessages)
	}
	st.mu.Unlock()
}

func TestMailboxTracker_QueueExpunge(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	mt.QueueExpunge(3)

	if mt.NumMessages() != 4 {
		t.Fatalf("NumMessages: expected %d, got %d", 4, mt.NumMessages())
	}

	// Check update was queued
	st.mu.Lock()
	if len(st.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.updates))
	}
	update, ok := st.updates[0].(ExpungeUpdate)
	if !ok {
		t.Fatalf("expected ExpungeUpdate, got %T", st.updates[0])
	}
	if update.SeqNum != 3 {
		t.Fatalf("ExpungeUpdate.SeqNum: expected %d, got %d", 3, update.SeqNum)
	}
	st.mu.Unlock()
}

func TestMailboxTracker_QueueExpunge_ZeroMessages(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 0, 1, 1)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	mt.QueueExpunge(1)

	// Should not go below zero
	if mt.NumMessages() != 0 {
		t.Fatalf("NumMessages: expected %d, got %d", 0, mt.NumMessages())
	}
}

func TestMailboxTracker_QueueFlagsUpdate(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	flags := []imap.Flag{imap.FlagSeen, imap.FlagFlagged}
	mt.QueueFlagsUpdate(2, flags)

	st.mu.Lock()
	if len(st.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.updates))
	}
	update, ok := st.updates[0].(FetchFlagsUpdate)
	if !ok {
		t.Fatalf("expected FetchFlagsUpdate, got %T", st.updates[0])
	}
	if update.SeqNum != 2 {
		t.Fatalf("FetchFlagsUpdate.SeqNum: expected %d, got %d", 2, update.SeqNum)
	}
	if len(update.Flags) != 2 {
		t.Fatalf("FetchFlagsUpdate.Flags: expected 2 flags, got %d", len(update.Flags))
	}
	st.mu.Unlock()
}

func TestMailboxTracker_QueueUpdate_MultipleSessions(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 0, 1, 1)
	st1 := NewSessionTracker()
	st2 := NewSessionTracker()
	st1.Select(mt)
	st2.Select(mt)
	defer st1.Unselect()
	defer st2.Unselect()

	mt.QueueNewMessage()

	st1.mu.Lock()
	if len(st1.updates) != 1 {
		t.Fatalf("st1: expected 1 update, got %d", len(st1.updates))
	}
	st1.mu.Unlock()

	st2.mu.Lock()
	if len(st2.updates) != 1 {
		t.Fatalf("st2: expected 1 update, got %d", len(st2.updates))
	}
	st2.mu.Unlock()
}

func TestMailboxTracker_QueueUpdate_NoSessions(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 0, 1, 1)

	// Should not panic when no sessions are watching
	mt.QueueNewMessage()
	mt.QueueExpunge(1)
	mt.QueueFlagsUpdate(1, []imap.Flag{imap.FlagSeen})

	if mt.NumMessages() != 0 {
		t.Fatalf("NumMessages: expected %d, got %d", 0, mt.NumMessages())
	}
}

// --- SessionTracker tests ---

func TestNewSessionTracker(t *testing.T) {
	st := NewSessionTracker()
	if st == nil {
		t.Fatal("NewSessionTracker returned nil")
	}
	if st.mailbox != nil {
		t.Fatal("expected nil mailbox initially")
	}
	if len(st.updates) != 0 {
		t.Fatalf("expected 0 updates initially, got %d", len(st.updates))
	}
}

func TestSessionTracker_Select(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()

	st.Select(mt)

	st.mu.Lock()
	if st.mailbox != mt {
		t.Fatal("mailbox not set correctly after Select")
	}
	st.mu.Unlock()

	// Verify session was added to the mailbox tracker
	mt.mu.RLock()
	if _, ok := mt.sessions[st]; !ok {
		t.Fatal("session not found in mailbox tracker after Select")
	}
	mt.mu.RUnlock()

	st.Unselect()
}

func TestSessionTracker_Select_ReplacesExisting(t *testing.T) {
	mt1 := NewMailboxTracker("INBOX", 5, 1, 10)
	mt2 := NewMailboxTracker("Sent", 3, 2, 5)
	st := NewSessionTracker()

	st.Select(mt1)

	// Queue an update in mt1
	mt1.QueueNewMessage()
	st.mu.Lock()
	if len(st.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.updates))
	}
	st.mu.Unlock()

	// Select mt2 should remove from mt1 and clear updates
	st.Select(mt2)

	st.mu.Lock()
	if st.mailbox != mt2 {
		t.Fatal("mailbox not set to mt2 after second Select")
	}
	if len(st.updates) != 0 {
		t.Fatalf("expected 0 updates after new Select, got %d", len(st.updates))
	}
	st.mu.Unlock()

	// Verify removed from mt1
	mt1.mu.RLock()
	if _, ok := mt1.sessions[st]; ok {
		t.Fatal("session should have been removed from mt1")
	}
	mt1.mu.RUnlock()

	// Verify added to mt2
	mt2.mu.RLock()
	if _, ok := mt2.sessions[st]; !ok {
		t.Fatal("session should have been added to mt2")
	}
	mt2.mu.RUnlock()

	st.Unselect()
}

func TestSessionTracker_Unselect(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()

	st.Select(mt)
	mt.QueueNewMessage()

	st.Unselect()

	st.mu.Lock()
	if st.mailbox != nil {
		t.Fatal("mailbox should be nil after Unselect")
	}
	if len(st.updates) != 0 {
		t.Fatalf("expected 0 updates after Unselect, got %d", len(st.updates))
	}
	st.mu.Unlock()

	// Verify removed from mailbox tracker
	mt.mu.RLock()
	if _, ok := mt.sessions[st]; ok {
		t.Fatal("session should have been removed from mailbox tracker after Unselect")
	}
	mt.mu.RUnlock()
}

func TestSessionTracker_Unselect_WhenNotSelected(t *testing.T) {
	st := NewSessionTracker()

	// Should not panic
	st.Unselect()

	st.mu.Lock()
	if st.mailbox != nil {
		t.Fatal("mailbox should be nil")
	}
	st.mu.Unlock()
}

func TestSessionTracker_Flush(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	// Queue multiple updates
	mt.QueueNewMessage()
	mt.QueueFlagsUpdate(2, []imap.Flag{imap.FlagSeen})

	// Create a buffer and an update writer
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := NewResponseEncoder(enc)
	w := NewUpdateWriter(respEnc)

	st.Flush(w, true)

	// After flush, updates should be cleared
	st.mu.Lock()
	if len(st.updates) != 0 {
		t.Fatalf("expected 0 updates after Flush, got %d", len(st.updates))
	}
	st.mu.Unlock()

	// Verify something was written (the buffer should not be empty)
	if buf.Len() == 0 {
		t.Fatal("expected some data written to buffer after Flush")
	}
}

func TestSessionTracker_Flush_AllowExpungeFalse(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	// Queue an expunge and an exists update
	mt.QueueExpunge(3)
	mt.QueueNewMessage()

	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := NewResponseEncoder(enc)
	w := NewUpdateWriter(respEnc)

	st.Flush(w, false)

	// After flush, updates should be cleared
	st.mu.Lock()
	if len(st.updates) != 0 {
		t.Fatalf("expected 0 updates after Flush, got %d", len(st.updates))
	}
	st.mu.Unlock()

	output := buf.String()
	// The EXISTS update should be written
	if !bytes.Contains([]byte(output), []byte("EXISTS")) {
		t.Fatal("expected EXISTS in output")
	}
	// The EXPUNGE update should NOT be written when allowExpunge is false
	if bytes.Contains([]byte(output), []byte("EXPUNGE")) {
		t.Fatal("EXPUNGE should not appear in output when allowExpunge is false")
	}
}

func TestSessionTracker_Flush_AllowExpungeTrue(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	mt.QueueExpunge(3)

	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := NewResponseEncoder(enc)
	w := NewUpdateWriter(respEnc)

	st.Flush(w, true)

	output := buf.String()
	// The EXPUNGE update should be written
	if !bytes.Contains([]byte(output), []byte("EXPUNGE")) {
		t.Fatalf("expected EXPUNGE in output, got %q", output)
	}
}

func TestSessionTracker_Flush_EmptyUpdates(t *testing.T) {
	st := NewSessionTracker()

	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := NewResponseEncoder(enc)
	w := NewUpdateWriter(respEnc)

	// Flushing with no updates should be a no-op
	st.Flush(w, true)

	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer, got %d bytes", buf.Len())
	}
}

func TestSessionTracker_QueueUpdate(t *testing.T) {
	st := NewSessionTracker()

	st.queueUpdate(ExistsUpdate{NumMessages: 10})
	st.queueUpdate(ExpungeUpdate{SeqNum: 5})
	st.queueUpdate(FetchFlagsUpdate{SeqNum: 2, Flags: []imap.Flag{imap.FlagSeen}})

	st.mu.Lock()
	if len(st.updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(st.updates))
	}
	st.mu.Unlock()
}

// --- Update type tests ---

func TestExistsUpdate_Type(t *testing.T) {
	u := ExistsUpdate{NumMessages: 10}
	if u.updateType() != "EXISTS" {
		t.Fatalf("expected %q, got %q", "EXISTS", u.updateType())
	}
}

func TestExpungeUpdate_Type(t *testing.T) {
	u := ExpungeUpdate{SeqNum: 5}
	if u.updateType() != "EXPUNGE" {
		t.Fatalf("expected %q, got %q", "EXPUNGE", u.updateType())
	}
}

func TestFetchFlagsUpdate_Type(t *testing.T) {
	u := FetchFlagsUpdate{SeqNum: 2, Flags: []imap.Flag{imap.FlagSeen}}
	if u.updateType() != "FETCH" {
		t.Fatalf("expected %q, got %q", "FETCH", u.updateType())
	}
}

func TestMailboxTracker_MultipleQueueNewMessage(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 0, 1, 1)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	mt.QueueNewMessage()
	mt.QueueNewMessage()
	mt.QueueNewMessage()

	if mt.NumMessages() != 3 {
		t.Fatalf("NumMessages: expected %d, got %d", 3, mt.NumMessages())
	}

	st.mu.Lock()
	if len(st.updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(st.updates))
	}

	// Verify the sequence
	for i, update := range st.updates {
		eu, ok := update.(ExistsUpdate)
		if !ok {
			t.Fatalf("update %d: expected ExistsUpdate, got %T", i, update)
		}
		expected := uint32(i + 1)
		if eu.NumMessages != expected {
			t.Fatalf("update %d: expected NumMessages=%d, got %d", i, expected, eu.NumMessages)
		}
	}
	st.mu.Unlock()
}

func TestMailboxTracker_AddAndRemoveSession(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 0, 1, 1)
	st1 := NewSessionTracker()
	st2 := NewSessionTracker()

	st1.Select(mt)
	st2.Select(mt)

	mt.mu.RLock()
	if len(mt.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(mt.sessions))
	}
	mt.mu.RUnlock()

	st1.Unselect()

	mt.mu.RLock()
	if len(mt.sessions) != 1 {
		t.Fatalf("expected 1 session after unselect, got %d", len(mt.sessions))
	}
	mt.mu.RUnlock()

	st2.Unselect()

	mt.mu.RLock()
	if len(mt.sessions) != 0 {
		t.Fatalf("expected 0 sessions after all unselected, got %d", len(mt.sessions))
	}
	mt.mu.RUnlock()
}

func TestSessionTracker_SelectNil(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()

	st.Select(mt)

	// Selecting nil should unselect from current mailbox
	st.Select(nil)

	mt.mu.RLock()
	if _, ok := mt.sessions[st]; ok {
		t.Fatal("session should have been removed from mailbox after Select(nil)")
	}
	mt.mu.RUnlock()

	st.mu.Lock()
	if st.mailbox != nil {
		t.Fatal("mailbox should be nil after Select(nil)")
	}
	st.mu.Unlock()
}

func TestSessionTracker_Flush_MixedUpdates(t *testing.T) {
	mt := NewMailboxTracker("INBOX", 5, 1, 10)
	st := NewSessionTracker()
	st.Select(mt)
	defer st.Unselect()

	// Queue different types of updates
	mt.QueueNewMessage()                                             // EXISTS
	mt.QueueFlagsUpdate(1, []imap.Flag{imap.FlagSeen})              // FETCH FLAGS
	mt.QueueExpunge(2)                                               // EXPUNGE
	mt.QueueFlagsUpdate(3, []imap.Flag{imap.FlagFlagged})           // FETCH FLAGS

	st.mu.Lock()
	if len(st.updates) != 4 {
		t.Fatalf("expected 4 updates, got %d", len(st.updates))
	}
	st.mu.Unlock()

	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := NewResponseEncoder(enc)
	w := NewUpdateWriter(respEnc)

	st.Flush(w, true)

	output := buf.String()

	// Verify all update types are present
	if !bytes.Contains([]byte(output), []byte("EXISTS")) {
		t.Fatal("expected EXISTS in output")
	}
	if !bytes.Contains([]byte(output), []byte("EXPUNGE")) {
		t.Fatal("expected EXPUNGE in output")
	}
	if !bytes.Contains([]byte(output), []byte("FETCH")) {
		t.Fatal("expected FETCH in output")
	}
}
