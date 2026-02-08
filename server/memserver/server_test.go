package memserver

import (
	"testing"
)

func TestNew(t *testing.T) {
	ms := New()
	if ms == nil {
		t.Fatal("New returned nil")
	}
	if ms.users == nil {
		t.Fatal("users map is nil")
	}
	if ms.userData == nil {
		t.Fatal("userData map is nil")
	}
	if len(ms.users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(ms.users))
	}
}

func TestAddUser(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "password123")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if pw, ok := ms.users["alice"]; !ok {
		t.Fatal("user alice not found")
	} else if pw != "password123" {
		t.Fatalf("expected password %q, got %q", "password123", pw)
	}

	ud, ok := ms.userData["alice"]
	if !ok {
		t.Fatal("user data for alice not found")
	}
	if ud == nil {
		t.Fatal("user data for alice is nil")
	}

	// Check that INBOX was created
	if _, exists := ud.Mailboxes["INBOX"]; !exists {
		t.Fatal("INBOX not created for new user")
	}
}

func TestAddUser_UpdatesPassword(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "oldpass")
	ms.AddUser("alice", "newpass")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if pw := ms.users["alice"]; pw != "newpass" {
		t.Fatalf("expected password %q, got %q", "newpass", pw)
	}
}

func TestAddUser_DoesNotResetUserData(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass1")

	// Get the user data and verify it exists
	ud1 := ms.GetUserData("alice")
	if ud1 == nil {
		t.Fatal("expected user data")
	}

	// Update password - should not reset user data
	ms.AddUser("alice", "pass2")

	ud2 := ms.GetUserData("alice")
	if ud2 != ud1 {
		t.Fatal("user data was reset when updating password")
	}
}

func TestAddUser_MultipleUsers(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass1")
	ms.AddUser("bob", "pass2")
	ms.AddUser("charlie", "pass3")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if len(ms.users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(ms.users))
	}
}

func TestRemoveUser(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass1")
	ms.AddUser("bob", "pass2")

	ms.RemoveUser("alice")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if _, ok := ms.users["alice"]; ok {
		t.Fatal("user alice should have been removed")
	}
	if _, ok := ms.userData["alice"]; ok {
		t.Fatal("user data for alice should have been removed")
	}
	if _, ok := ms.users["bob"]; !ok {
		t.Fatal("user bob should still exist")
	}
}

func TestRemoveUser_NonExistent(t *testing.T) {
	ms := New()

	// Should not panic
	ms.RemoveUser("nonexistent")
}

func TestGetUserData(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass")

	ud := ms.GetUserData("alice")
	if ud == nil {
		t.Fatal("expected user data, got nil")
	}

	// Should have INBOX
	inbox := ud.GetMailbox("INBOX")
	if inbox == nil {
		t.Fatal("expected INBOX in user data")
	}
}

func TestGetUserData_NonExistentUser(t *testing.T) {
	ms := New()

	ud := ms.GetUserData("nonexistent")
	if ud != nil {
		t.Fatalf("expected nil for non-existent user, got %v", ud)
	}
}

func TestNewSession(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass")

	sess, err := ms.NewSession(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("session is nil")
	}

	// The session should be of type *Session
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("expected *Session, got %T", sess)
	}

	// The session should reference the MemServer
	if s.srv != ms {
		t.Fatal("session does not reference the correct MemServer")
	}

	// Session should not be authenticated yet
	if s.userData != nil {
		t.Fatal("session should not have user data before login")
	}
}

func TestNewServer(t *testing.T) {
	ms := New()
	srv := ms.NewServer()

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	opts := srv.Options()
	if opts == nil {
		t.Fatal("server options are nil")
	}
	if !opts.AllowInsecureAuth {
		t.Fatal("expected AllowInsecureAuth to be true")
	}
	if opts.NewSession == nil {
		t.Fatal("expected NewSession callback to be set")
	}
}

// --- UserData tests ---

func TestNewUserData(t *testing.T) {
	ud := NewUserData()

	if ud == nil {
		t.Fatal("NewUserData returned nil")
	}
	if ud.Mailboxes == nil {
		t.Fatal("Mailboxes map is nil")
	}
	if len(ud.Mailboxes) != 1 {
		t.Fatalf("expected 1 mailbox (INBOX), got %d", len(ud.Mailboxes))
	}

	inbox, ok := ud.Mailboxes["INBOX"]
	if !ok {
		t.Fatal("INBOX not found")
	}
	if !inbox.Subscribed {
		t.Fatal("INBOX should be subscribed by default")
	}
}

func TestUserData_GetMailbox(t *testing.T) {
	ud := NewUserData()

	inbox := ud.GetMailbox("INBOX")
	if inbox == nil {
		t.Fatal("expected INBOX, got nil")
	}
}

func TestUserData_GetMailbox_CaseInsensitiveInbox(t *testing.T) {
	ud := NewUserData()

	tests := []string{"INBOX", "inbox", "Inbox", "InBoX"}
	for _, name := range tests {
		mbox := ud.GetMailbox(name)
		if mbox == nil {
			t.Fatalf("GetMailbox(%q) returned nil, expected INBOX", name)
		}
	}
}

func TestUserData_GetMailbox_NonExistent(t *testing.T) {
	ud := NewUserData()

	mbox := ud.GetMailbox("NonExistent")
	if mbox != nil {
		t.Fatalf("expected nil for non-existent mailbox, got %v", mbox)
	}
}

func TestUserData_CreateMailbox(t *testing.T) {
	ud := NewUserData()

	err := ud.CreateMailbox("Sent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mbox := ud.GetMailbox("Sent")
	if mbox == nil {
		t.Fatal("mailbox Sent not found after creation")
	}
	if mbox.Name != "Sent" {
		t.Fatalf("expected name %q, got %q", "Sent", mbox.Name)
	}
}

func TestUserData_CreateMailbox_AlreadyExists(t *testing.T) {
	ud := NewUserData()

	err := ud.CreateMailbox("INBOX")
	if err != ErrMailboxAlreadyExists {
		t.Fatalf("expected ErrMailboxAlreadyExists, got %v", err)
	}
}

func TestUserData_CreateMailbox_DuplicateName(t *testing.T) {
	ud := NewUserData()

	err := ud.CreateMailbox("Sent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = ud.CreateMailbox("Sent")
	if err != ErrMailboxAlreadyExists {
		t.Fatalf("expected ErrMailboxAlreadyExists, got %v", err)
	}
}

func TestUserData_DeleteMailbox(t *testing.T) {
	ud := NewUserData()
	_ = ud.CreateMailbox("Sent")

	err := ud.DeleteMailbox("Sent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mbox := ud.GetMailbox("Sent")
	if mbox != nil {
		t.Fatal("mailbox Sent should have been deleted")
	}
}

func TestUserData_DeleteMailbox_INBOX(t *testing.T) {
	ud := NewUserData()

	err := ud.DeleteMailbox("INBOX")
	if err == nil {
		t.Fatal("expected error when deleting INBOX")
	}
}

func TestUserData_DeleteMailbox_CaseInsensitiveINBOX(t *testing.T) {
	ud := NewUserData()

	err := ud.DeleteMailbox("inbox")
	if err == nil {
		t.Fatal("expected error when deleting inbox (case-insensitive INBOX)")
	}
}

func TestUserData_DeleteMailbox_NonExistent(t *testing.T) {
	ud := NewUserData()

	err := ud.DeleteMailbox("NonExistent")
	if err != ErrNoSuchMailbox {
		t.Fatalf("expected ErrNoSuchMailbox, got %v", err)
	}
}

func TestUserData_RenameMailbox(t *testing.T) {
	ud := NewUserData()
	_ = ud.CreateMailbox("OldName")

	err := ud.RenameMailbox("OldName", "NewName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	old := ud.GetMailbox("OldName")
	if old != nil {
		t.Fatal("old mailbox should not exist after rename")
	}

	newMbox := ud.GetMailbox("NewName")
	if newMbox == nil {
		t.Fatal("new mailbox should exist after rename")
	}
	if newMbox.Name != "NewName" {
		t.Fatalf("expected name %q, got %q", "NewName", newMbox.Name)
	}
}

func TestUserData_RenameMailbox_NonExistent(t *testing.T) {
	ud := NewUserData()

	err := ud.RenameMailbox("NonExistent", "NewName")
	if err != ErrNoSuchMailbox {
		t.Fatalf("expected ErrNoSuchMailbox, got %v", err)
	}
}

func TestUserData_RenameMailbox_DestinationExists(t *testing.T) {
	ud := NewUserData()
	_ = ud.CreateMailbox("Sent")
	_ = ud.CreateMailbox("Drafts")

	err := ud.RenameMailbox("Sent", "Drafts")
	if err != ErrMailboxAlreadyExists {
		t.Fatalf("expected ErrMailboxAlreadyExists, got %v", err)
	}
}

func TestUserData_MailboxNames(t *testing.T) {
	ud := NewUserData()
	_ = ud.CreateMailbox("Sent")
	_ = ud.CreateMailbox("Drafts")

	names := ud.MailboxNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 mailbox names, got %d: %v", len(names), names)
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"INBOX", "Sent", "Drafts"} {
		if !nameSet[expected] {
			t.Fatalf("expected %q in mailbox names", expected)
		}
	}
}

// --- normalizeINBOX tests ---

func TestNormalizeINBOX(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INBOX", "INBOX"},
		{"inbox", "INBOX"},
		{"Inbox", "INBOX"},
		{"InBoX", "INBOX"},
		{"inBOX", "INBOX"},
		{"Sent", "Sent"},
		{"inbox2", "inbox2"},
		{"INBO", "INBO"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeINBOX(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeINBOX(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- IMAPError tests ---

func TestIMAPError(t *testing.T) {
	e := &IMAPError{Message: "test error"}
	if e.Error() != "test error" {
		t.Fatalf("expected %q, got %q", "test error", e.Error())
	}
}
