package memserver

import (
	"bytes"
	"strings"
	"testing"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// helper to create a logged-in session
func newLoggedInSession(t *testing.T) (*Session, *MemServer) {
	t.Helper()
	ms := New()
	ms.AddUser("alice", "password123")
	s := &Session{srv: ms}
	if err := s.Login("alice", "password123"); err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	return s, ms
}

// helper to create a session with a selected mailbox
func newSelectedSession(t *testing.T) (*Session, *MemServer) {
	t.Helper()
	s, ms := newLoggedInSession(t)
	_, err := s.Select("INBOX", nil)
	if err != nil {
		t.Fatalf("failed to select INBOX: %v", err)
	}
	return s, ms
}

// helper to create a no-op ExpungeWriter
func newExpungeWriter() *server.ExpungeWriter {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := server.NewResponseEncoder(enc)
	return server.NewExpungeWriter(respEnc)
}

// helper to create a no-op FetchWriter
func newFetchWriter() *server.FetchWriter {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := server.NewResponseEncoder(enc)
	return server.NewFetchWriter(respEnc)
}

// helper to create an UpdateWriter
func newUpdateWriter() *server.UpdateWriter {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	respEnc := server.NewResponseEncoder(enc)
	return server.NewUpdateWriter(respEnc)
}

// helper to create a ListWriter and capture buffer
func newListWriterWithBuffer() (*server.ListWriter, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	enc := wire.NewEncoder(buf)
	respEnc := server.NewResponseEncoder(enc)
	return server.NewListWriter(respEnc), buf
}

// --- Login tests ---

func TestSession_Login_Success(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass123")
	s := &Session{srv: ms}

	err := s.Login("alice", "pass123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.userData == nil {
		t.Fatal("userData should be set after successful login")
	}
}

func TestSession_Login_WrongPassword(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass123")
	s := &Session{srv: ms}

	err := s.Login("alice", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if s.userData != nil {
		t.Fatal("userData should not be set after failed login")
	}
}

func TestSession_Login_NonExistentUser(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Login("nonexistent", "pass")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestSession_Login_EmptyCredentials(t *testing.T) {
	ms := New()
	ms.AddUser("alice", "pass")
	s := &Session{srv: ms}

	err := s.Login("", "")
	if err == nil {
		t.Fatal("expected error for empty credentials")
	}
}

// --- Close tests ---

func TestSession_Close(t *testing.T) {
	s, _ := newSelectedSession(t)

	err := s.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.selectedMailbox != nil {
		t.Fatal("selectedMailbox should be nil after Close")
	}
	if s.userData != nil {
		t.Fatal("userData should be nil after Close")
	}
}

// --- Select tests ---

func TestSession_Select(t *testing.T) {
	s, _ := newLoggedInSession(t)

	data, err := s.Select("INBOX", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected SelectData, got nil")
	}
	if data.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %d", data.UIDValidity)
	}
	if s.selectedMailbox == nil {
		t.Fatal("selectedMailbox should be set after Select")
	}
	if s.selectedReadOnly {
		t.Fatal("expected selectedReadOnly false")
	}
}

func TestSession_Select_ReadOnly(t *testing.T) {
	s, _ := newLoggedInSession(t)

	opts := &imap.SelectOptions{ReadOnly: true}
	data, err := s.Select("INBOX", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !data.ReadOnly {
		t.Fatal("expected ReadOnly true")
	}
	if !s.selectedReadOnly {
		t.Fatal("expected selectedReadOnly true")
	}
}

func TestSession_Select_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_, err := s.Select("NonExistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent mailbox")
	}
}

func TestSession_Select_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	_, err := s.Select("INBOX", nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Create tests ---

func TestSession_Create(t *testing.T) {
	s, ms := newLoggedInSession(t)

	err := s.Create("Sent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ud := ms.GetUserData("alice")
	mbox := ud.GetMailbox("Sent")
	if mbox == nil {
		t.Fatal("mailbox Sent should exist after Create")
	}
}

func TestSession_Create_AlreadyExists(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Create("INBOX", nil)
	if err == nil {
		t.Fatal("expected error when creating existing mailbox")
	}
}

func TestSession_Create_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Create("Test", nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Delete tests ---

func TestSession_Delete(t *testing.T) {
	s, ms := newLoggedInSession(t)

	_ = s.Create("ToDelete", nil)

	err := s.Delete("ToDelete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ud := ms.GetUserData("alice")
	if ud.GetMailbox("ToDelete") != nil {
		t.Fatal("mailbox should have been deleted")
	}
}

func TestSession_Delete_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Delete("NonExistent")
	if err == nil {
		t.Fatal("expected error deleting non-existent mailbox")
	}
}

func TestSession_Delete_INBOX(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Delete("INBOX")
	if err == nil {
		t.Fatal("expected error deleting INBOX")
	}
}

func TestSession_Delete_CurrentlySelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("TestMbox", nil)
	_, _ = s.Select("TestMbox", nil)

	err := s.Delete("TestMbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.selectedMailbox != nil {
		t.Fatal("selectedMailbox should be cleared when selected mailbox is deleted")
	}
}

func TestSession_Delete_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Delete("INBOX")
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Rename tests ---

func TestSession_Rename(t *testing.T) {
	s, ms := newLoggedInSession(t)

	_ = s.Create("OldName", nil)

	err := s.Rename("OldName", "NewName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ud := ms.GetUserData("alice")
	if ud.GetMailbox("OldName") != nil {
		t.Fatal("old mailbox name should not exist")
	}
	if ud.GetMailbox("NewName") == nil {
		t.Fatal("new mailbox name should exist")
	}
}

func TestSession_Rename_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Rename("NonExistent", "NewName")
	if err == nil {
		t.Fatal("expected error renaming non-existent mailbox")
	}
}

func TestSession_Rename_DestinationExists(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("Source", nil)
	_ = s.Create("Destination", nil)

	err := s.Rename("Source", "Destination")
	if err == nil {
		t.Fatal("expected error when destination exists")
	}
}

func TestSession_Rename_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Rename("Old", "New")
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Subscribe tests ---

func TestSession_Subscribe(t *testing.T) {
	s, ms := newLoggedInSession(t)

	_ = s.Create("TestMailbox", nil)

	err := s.Subscribe("TestMailbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ud := ms.GetUserData("alice")
	mbox := ud.GetMailbox("TestMailbox")
	if !mbox.Subscribed {
		t.Fatal("mailbox should be subscribed")
	}
}

func TestSession_Subscribe_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Subscribe("NonExistent")
	if err == nil {
		t.Fatal("expected error subscribing to non-existent mailbox")
	}
}

func TestSession_Subscribe_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Subscribe("INBOX")
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Unsubscribe tests ---

func TestSession_Unsubscribe(t *testing.T) {
	s, ms := newLoggedInSession(t)

	// INBOX is subscribed by default
	err := s.Unsubscribe("INBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ud := ms.GetUserData("alice")
	mbox := ud.GetMailbox("INBOX")
	if mbox.Subscribed {
		t.Fatal("mailbox should be unsubscribed")
	}
}

func TestSession_Unsubscribe_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Unsubscribe("NonExistent")
	if err == nil {
		t.Fatal("expected error unsubscribing from non-existent mailbox")
	}
}

func TestSession_Unsubscribe_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	err := s.Unsubscribe("INBOX")
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- List tests ---

func TestSession_List_EmptyPattern(t *testing.T) {
	s, _ := newLoggedInSession(t)

	w, buf := newListWriterWithBuffer()

	err := s.List(w, "", []string{""}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "LIST") {
		t.Fatal("expected LIST response for empty pattern")
	}
}

func TestSession_List_AllMailboxes(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("Sent", nil)
	_ = s.Create("Drafts", nil)

	w, buf := newListWriterWithBuffer()

	err := s.List(w, "", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INBOX") {
		t.Fatal("expected INBOX in LIST response")
	}
	if !strings.Contains(output, "Sent") {
		t.Fatal("expected Sent in LIST response")
	}
	if !strings.Contains(output, "Drafts") {
		t.Fatal("expected Drafts in LIST response")
	}
}

func TestSession_List_WithSubscribed(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("Sent", nil)
	_ = s.Subscribe("Sent")
	_ = s.Create("Drafts", nil) // not subscribed

	w, buf := newListWriterWithBuffer()

	opts := &imap.ListOptions{
		SelectSubscribed: true,
	}
	err := s.List(w, "", []string{"*"}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// INBOX is subscribed by default
	if !strings.Contains(output, "INBOX") {
		t.Fatal("expected INBOX in subscribed LIST response")
	}
	if !strings.Contains(output, "Sent") {
		t.Fatal("expected Sent in subscribed LIST response")
	}
	if strings.Contains(output, "Drafts") {
		t.Fatal("Drafts should not appear in subscribed-only LIST response")
	}
}

func TestSession_List_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	w, _ := newListWriterWithBuffer()
	err := s.List(w, "", []string{"*"}, nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Status tests ---

func TestSession_Status(t *testing.T) {
	s, _ := newLoggedInSession(t)

	opts := &imap.StatusOptions{
		NumMessages: true,
		UIDNext:     true,
		UIDValidity: true,
		NumUnseen:   true,
	}

	data, err := s.Status("INBOX", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected StatusData, got nil")
	}
	if data.Mailbox != "INBOX" {
		t.Fatalf("expected mailbox %q, got %q", "INBOX", data.Mailbox)
	}
	if data.NumMessages == nil {
		t.Fatal("expected NumMessages to be set")
	}
}

func TestSession_Status_NonExistent(t *testing.T) {
	s, _ := newLoggedInSession(t)

	opts := &imap.StatusOptions{NumMessages: true}
	_, err := s.Status("NonExistent", opts)
	if err == nil {
		t.Fatal("expected error for non-existent mailbox")
	}
}

func TestSession_Status_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	opts := &imap.StatusOptions{NumMessages: true}
	_, err := s.Status("INBOX", opts)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Append tests ---

func TestSession_Append(t *testing.T) {
	s, ms := newLoggedInSession(t)

	body := []byte("From: test@example.com\r\nSubject: Test\r\n\r\nBody")
	r := imap.LiteralReader{Reader: bytes.NewReader(body), Size: int64(len(body))}

	opts := &imap.AppendOptions{
		Flags:        []imap.Flag{imap.FlagSeen},
		InternalDate: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	data, err := s.Append("INBOX", r, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected AppendData, got nil")
	}
	if data.UID != 1 {
		t.Fatalf("expected UID 1, got %d", data.UID)
	}
	if data.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %d", data.UIDValidity)
	}

	// Verify message was actually stored
	ud := ms.GetUserData("alice")
	mbox := ud.GetMailbox("INBOX")
	if mbox.NumMessages() != 1 {
		t.Fatalf("expected 1 message, got %d", mbox.NumMessages())
	}
}

func TestSession_Append_NonExistentMailbox(t *testing.T) {
	s, _ := newLoggedInSession(t)

	body := []byte("body")
	r := imap.LiteralReader{Reader: bytes.NewReader(body), Size: int64(len(body))}

	_, err := s.Append("NonExistent", r, nil)
	if err == nil {
		t.Fatal("expected error appending to non-existent mailbox")
	}
}

func TestSession_Append_NilOptions(t *testing.T) {
	s, _ := newLoggedInSession(t)

	body := []byte("body")
	r := imap.LiteralReader{Reader: bytes.NewReader(body), Size: int64(len(body))}

	data, err := s.Append("INBOX", r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.UID != 1 {
		t.Fatalf("expected UID 1, got %d", data.UID)
	}
}

func TestSession_Append_MultipleMessages(t *testing.T) {
	s, _ := newLoggedInSession(t)

	for i := 0; i < 3; i++ {
		body := []byte("message body")
		r := imap.LiteralReader{Reader: bytes.NewReader(body), Size: int64(len(body))}
		_, err := s.Append("INBOX", r, nil)
		if err != nil {
			t.Fatalf("unexpected error on message %d: %v", i, err)
		}
	}

	// Verify all messages stored
	data, err := s.Status("INBOX", &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *data.NumMessages != 3 {
		t.Fatalf("expected 3 messages, got %d", *data.NumMessages)
	}
}

func TestSession_Append_NotAuthenticated(t *testing.T) {
	ms := New()
	s := &Session{srv: ms}

	body := []byte("body")
	r := imap.LiteralReader{Reader: bytes.NewReader(body), Size: int64(len(body))}

	_, err := s.Append("INBOX", r, nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

// --- Search tests ---

func TestSession_Search_SeqNum(t *testing.T) {
	s, _ := newSelectedSession(t)

	// Append some messages
	appendTestMessage(t, s, "INBOX", "msg1", nil)
	appendTestMessage(t, s, "INBOX", "msg2", []imap.Flag{imap.FlagSeen})
	appendTestMessage(t, s, "INBOX", "msg3", nil)

	// Re-select after appending
	_, _ = s.Select("INBOX", nil)

	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}

	data, err := s.Search(imap.NumKindSeq, criteria, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.AllSeqNums) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(data.AllSeqNums), data.AllSeqNums)
	}
}

func TestSession_Search_UID(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagSeen})
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", []imap.Flag{imap.FlagSeen})

	_, _ = s.Select("INBOX", nil)

	criteria := &imap.SearchCriteria{
		Flag: []imap.Flag{imap.FlagSeen},
	}

	data, err := s.Search(imap.NumKindUID, criteria, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.AllUIDs) != 2 {
		t.Fatalf("expected 2 UIDs, got %d: %v", len(data.AllUIDs), data.AllUIDs)
	}
}

func TestSession_Search_WithReturnOptions(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", nil)

	_, _ = s.Select("INBOX", nil)

	options := &imap.SearchOptions{
		ReturnCount: true,
		ReturnMin:   true,
		ReturnMax:   true,
		ReturnAll:   true,
	}

	data, err := s.Search(imap.NumKindSeq, nil, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Count != 3 {
		t.Fatalf("expected count 3, got %d", data.Count)
	}
	if data.Min != 1 {
		t.Fatalf("expected min 1, got %d", data.Min)
	}
	if data.Max != 3 {
		t.Fatalf("expected max 3, got %d", data.Max)
	}
	if data.All == nil {
		t.Fatal("expected All set")
	}
}

func TestSession_Search_NoMailboxSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_, err := s.Search(imap.NumKindSeq, nil, nil)
	if err == nil {
		t.Fatal("expected error when no mailbox selected")
	}
}

// --- Fetch tests ---

func TestSession_Fetch_Flags(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagSeen})
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		Flags: true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_UID(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	uidSet := &imap.UIDSet{}
	uidSet.AddNum(1)

	opts := &imap.FetchOptions{
		UID: true,
	}

	err := s.Fetch(w, uidSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_Envelope(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\nSubject: Test Subject\r\n\r\nBody",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		Envelope: true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_RFC822Size(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "exactly 16 bytes", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		RFC822Size: true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_InternalDate(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "body", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		InternalDate: true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_MultipleItems(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\nSubject: Test\r\n\r\nBody content",
		[]imap.Flag{imap.FlagSeen})
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		UID:          true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		Envelope:     true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_BodySection(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\nSubject: Test\r\n\r\nBody content",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Specifier: "", Peek: true}, // entire message
		},
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_BodySectionHeader(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\nSubject: Test\r\n\r\nBody content",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Specifier: "HEADER", Peek: true},
		},
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_BodySectionText(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\nSubject: Test\r\n\r\nBody content",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Specifier: "TEXT", Peek: true},
		},
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSession_Fetch_BodySectionSetsSeen(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\n\r\nBody",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	// Non-peek should set \Seen
	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Specifier: "", Peek: false},
		},
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the message now has \Seen
	msg := s.selectedMailbox.Messages[0]
	if !msg.HasFlag(imap.FlagSeen) {
		t.Fatal("expected \\Seen flag to be set after non-peek fetch")
	}
}

func TestSession_Fetch_BodySectionPeekDoesNotSetSeen(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX",
		"From: alice@example.com\r\n\r\nBody",
		nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	// Peek should NOT set \Seen
	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Specifier: "", Peek: true},
		},
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := s.selectedMailbox.Messages[0]
	if msg.HasFlag(imap.FlagSeen) {
		t.Fatal("\\Seen flag should not be set after peek fetch")
	}
}

func TestSession_Fetch_NoMailboxSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	opts := &imap.FetchOptions{Flags: true}

	err := s.Fetch(w, seqSet, opts)
	if err == nil {
		t.Fatal("expected error when no mailbox selected")
	}
}

func TestSession_Fetch_MultipleMessages(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagSeen})
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", []imap.Flag{imap.FlagFlagged})
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddRange(1, 3)

	opts := &imap.FetchOptions{
		Flags: true,
		UID:   true,
	}

	err := s.Fetch(w, seqSet, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Store tests ---

func TestSession_Store_AddFlags(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := s.selectedMailbox.Messages[0]
	if !msg.HasFlag(imap.FlagSeen) {
		t.Fatal("expected \\Seen flag")
	}
	if !msg.HasFlag(imap.FlagFlagged) {
		t.Fatal("expected \\Flagged flag")
	}
}

func TestSession_Store_RemoveFlags(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", []imap.Flag{imap.FlagSeen, imap.FlagFlagged})
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsDel,
		Flags:  []imap.Flag{imap.FlagSeen},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := s.selectedMailbox.Messages[0]
	if msg.HasFlag(imap.FlagSeen) {
		t.Fatal("\\Seen flag should have been removed")
	}
	if !msg.HasFlag(imap.FlagFlagged) {
		t.Fatal("\\Flagged flag should still be present")
	}
}

func TestSession_Store_SetFlags(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", []imap.Flag{imap.FlagSeen, imap.FlagFlagged})
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsSet,
		Flags:  []imap.Flag{imap.FlagDraft},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := s.selectedMailbox.Messages[0]
	if msg.HasFlag(imap.FlagSeen) {
		t.Fatal("\\Seen flag should have been replaced")
	}
	if msg.HasFlag(imap.FlagFlagged) {
		t.Fatal("\\Flagged flag should have been replaced")
	}
	if !msg.HasFlag(imap.FlagDraft) {
		t.Fatal("\\Draft flag should be set")
	}
}

func TestSession_Store_Silent(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagSeen},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The flag should still be set even if silent
	msg := s.selectedMailbox.Messages[0]
	if !msg.HasFlag(imap.FlagSeen) {
		t.Fatal("expected \\Seen flag even in silent mode")
	}
}

func TestSession_Store_ReadOnly(t *testing.T) {
	s, _ := newLoggedInSession(t)

	appendTestMessage(t, s, "INBOX", "msg", nil)
	_, _ = s.Select("INBOX", &imap.SelectOptions{ReadOnly: true})

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err == nil {
		t.Fatal("expected error when mailbox is read-only")
	}
}

func TestSession_Store_NoMailboxSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err == nil {
		t.Fatal("expected error when no mailbox selected")
	}
}

func TestSession_Store_MultipleMessages(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	seqSet := &imap.SeqSet{}
	seqSet.AddRange(1, 3)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen},
	}

	err := s.Store(w, seqSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, msg := range s.selectedMailbox.Messages {
		if !msg.HasFlag(imap.FlagSeen) {
			t.Fatalf("message %d should have \\Seen flag", i+1)
		}
	}
}

func TestSession_Store_WithUIDSet(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", nil)
	_, _ = s.Select("INBOX", nil)

	w := newFetchWriter()
	uidSet := &imap.UIDSet{}
	uidSet.AddNum(2)

	flags := &imap.StoreFlags{
		Action: imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagFlagged},
	}

	err := s.Store(w, uidSet, flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only message with UID 2 should have the flag
	if s.selectedMailbox.Messages[0].HasFlag(imap.FlagFlagged) {
		t.Fatal("message 1 should not have \\Flagged")
	}
	if !s.selectedMailbox.Messages[1].HasFlag(imap.FlagFlagged) {
		t.Fatal("message 2 should have \\Flagged")
	}
	if s.selectedMailbox.Messages[2].HasFlag(imap.FlagFlagged) {
		t.Fatal("message 3 should not have \\Flagged")
	}
}

// --- Copy tests ---

func TestSession_Copy(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("Backup", nil)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagSeen})
	appendTestMessage(t, s, "INBOX", "msg2", nil)

	_, _ = s.Select("INBOX", nil)

	seqSet := &imap.SeqSet{}
	seqSet.AddRange(1, 2)

	data, err := s.Copy(seqSet, "Backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected CopyData, got nil")
	}
	if data.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %d", data.UIDValidity)
	}

	// Check that messages were copied
	backupMbox := s.userData.GetMailbox("Backup")
	if backupMbox.NumMessages() != 2 {
		t.Fatalf("expected 2 messages in Backup, got %d", backupMbox.NumMessages())
	}

	// Original messages should still exist
	if s.selectedMailbox.NumMessages() != 2 {
		t.Fatalf("expected 2 messages in INBOX, got %d", s.selectedMailbox.NumMessages())
	}
}

func TestSession_Copy_WithUIDSet(t *testing.T) {
	s, _ := newLoggedInSession(t)

	_ = s.Create("Backup", nil)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", nil)

	_, _ = s.Select("INBOX", nil)

	uidSet := &imap.UIDSet{}
	uidSet.AddNum(1, 3)

	data, err := s.Copy(uidSet, "Backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	backupMbox := s.userData.GetMailbox("Backup")
	if backupMbox.NumMessages() != 2 {
		t.Fatalf("expected 2 messages in Backup, got %d", backupMbox.NumMessages())
	}

	// Check that SourceUIDs contains the right UIDs
	if !data.SourceUIDs.Contains(1) || !data.SourceUIDs.Contains(3) {
		t.Fatal("expected source UIDs to contain 1 and 3")
	}
}

func TestSession_Copy_NonExistentDest(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", nil)
	_, _ = s.Select("INBOX", nil)

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	_, err := s.Copy(seqSet, "NonExistent")
	if err == nil {
		t.Fatal("expected error copying to non-existent mailbox")
	}
}

func TestSession_Copy_NoMailboxSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	_, err := s.Copy(seqSet, "INBOX")
	if err == nil {
		t.Fatal("expected error when no mailbox selected")
	}
}

func TestSession_Copy_ToSameMailbox(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg", nil)
	_, _ = s.Select("INBOX", nil)

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	data, err := s.Copy(seqSet, "INBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should now have 2 messages in INBOX
	if s.selectedMailbox.NumMessages() != 2 {
		t.Fatalf("expected 2 messages after copy to same, got %d", s.selectedMailbox.NumMessages())
	}
	if data.DestUIDs.IsEmpty() {
		t.Fatal("expected non-empty DestUIDs")
	}
}

// --- Expunge tests ---

func TestSession_Expunge(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagDeleted})
	appendTestMessage(t, s, "INBOX", "msg2", nil)
	appendTestMessage(t, s, "INBOX", "msg3", []imap.Flag{imap.FlagDeleted})

	_, _ = s.Select("INBOX", nil)

	w := newExpungeWriter()
	err := s.Expunge(w, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.selectedMailbox.NumMessages() != 1 {
		t.Fatalf("expected 1 remaining message, got %d", s.selectedMailbox.NumMessages())
	}
}

func TestSession_Expunge_WithUIDSet(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", []imap.Flag{imap.FlagDeleted}) // UID 1
	appendTestMessage(t, s, "INBOX", "msg2", []imap.Flag{imap.FlagDeleted}) // UID 2
	appendTestMessage(t, s, "INBOX", "msg3", []imap.Flag{imap.FlagDeleted}) // UID 3

	_, _ = s.Select("INBOX", nil)

	uidSet := &imap.UIDSet{}
	uidSet.AddNum(1, 3)

	w := newExpungeWriter()
	err := s.Expunge(w, uidSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only message with UID 2 should remain
	if s.selectedMailbox.NumMessages() != 1 {
		t.Fatalf("expected 1 remaining, got %d", s.selectedMailbox.NumMessages())
	}
	if s.selectedMailbox.Messages[0].UID != 2 {
		t.Fatalf("expected UID 2, got %d", s.selectedMailbox.Messages[0].UID)
	}
}

func TestSession_Expunge_NoDeletedMessages(t *testing.T) {
	s, _ := newSelectedSession(t)

	appendTestMessage(t, s, "INBOX", "msg1", nil)
	_, _ = s.Select("INBOX", nil)

	w := newExpungeWriter()
	err := s.Expunge(w, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.selectedMailbox.NumMessages() != 1 {
		t.Fatalf("expected 1 message, got %d", s.selectedMailbox.NumMessages())
	}
}

func TestSession_Expunge_NoMailboxSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	w := newExpungeWriter()
	err := s.Expunge(w, nil)
	if err == nil {
		t.Fatal("expected error when no mailbox selected")
	}
}

// --- Unselect tests ---

func TestSession_Unselect(t *testing.T) {
	s, _ := newSelectedSession(t)

	err := s.Unselect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.selectedMailbox != nil {
		t.Fatal("selectedMailbox should be nil after Unselect")
	}
	if s.selectedReadOnly {
		t.Fatal("selectedReadOnly should be false after Unselect")
	}
}

func TestSession_Unselect_WhenNoneSelected(t *testing.T) {
	s, _ := newLoggedInSession(t)

	err := s.Unselect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Poll tests ---

func TestSession_Poll(t *testing.T) {
	s, _ := newSelectedSession(t)

	w := newUpdateWriter()
	err := s.Poll(w, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Idle tests ---

func TestSession_Idle(t *testing.T) {
	s, _ := newSelectedSession(t)

	w := newUpdateWriter()
	stop := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- s.Idle(w, stop)
	}()

	close(stop)

	err := <-done
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- filterHeaders tests ---

func TestFilterHeaders_Include(t *testing.T) {
	headers := []byte("From: alice@example.com\r\nSubject: Test\r\nDate: Mon, 1 Jan 2024\r\n\r\n")

	result := filterHeaders(headers, []string{"Subject", "From"}, false)

	if !bytes.Contains(result, []byte("From:")) {
		t.Fatal("expected From header in result")
	}
	if !bytes.Contains(result, []byte("Subject:")) {
		t.Fatal("expected Subject header in result")
	}
	if bytes.Contains(result, []byte("Date:")) {
		t.Fatal("Date header should not be in result")
	}
}

func TestFilterHeaders_Exclude(t *testing.T) {
	headers := []byte("From: alice@example.com\r\nSubject: Test\r\nDate: Mon, 1 Jan 2024\r\n\r\n")

	result := filterHeaders(headers, []string{"Subject"}, true)

	if !bytes.Contains(result, []byte("From:")) {
		t.Fatal("expected From header in result")
	}
	if !bytes.Contains(result, []byte("Date:")) {
		t.Fatal("expected Date header in result")
	}
	if bytes.Contains(result, []byte("Subject: Test")) {
		t.Fatal("Subject header should not be in result")
	}
}

func TestFilterHeaders_CaseInsensitive(t *testing.T) {
	headers := []byte("from: alice@example.com\r\nSUBJECT: Test\r\n\r\n")

	result := filterHeaders(headers, []string{"FROM"}, false)

	if !bytes.Contains(result, []byte("from:")) {
		t.Fatal("expected from header in result (case-insensitive match)")
	}
}

// --- Message tests ---

func TestMessage_HasFlag(t *testing.T) {
	msg := &Message{
		Flags: []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
	}

	if !msg.HasFlag(imap.FlagSeen) {
		t.Fatal("expected HasFlag(\\Seen) = true")
	}
	if !msg.HasFlag(imap.FlagFlagged) {
		t.Fatal("expected HasFlag(\\Flagged) = true")
	}
	if msg.HasFlag(imap.FlagDeleted) {
		t.Fatal("expected HasFlag(\\Deleted) = false")
	}
}

func TestMessage_SetFlag(t *testing.T) {
	msg := &Message{}

	msg.SetFlag(imap.FlagSeen)
	if !msg.HasFlag(imap.FlagSeen) {
		t.Fatal("expected \\Seen after SetFlag")
	}

	// Setting the same flag again should be idempotent
	msg.SetFlag(imap.FlagSeen)
	if len(msg.Flags) != 1 {
		t.Fatalf("expected 1 flag after duplicate SetFlag, got %d", len(msg.Flags))
	}
}

func TestMessage_RemoveFlag(t *testing.T) {
	msg := &Message{
		Flags: []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
	}

	msg.RemoveFlag(imap.FlagSeen)
	if msg.HasFlag(imap.FlagSeen) {
		t.Fatal("\\Seen should have been removed")
	}
	if !msg.HasFlag(imap.FlagFlagged) {
		t.Fatal("\\Flagged should still be present")
	}
}

func TestMessage_RemoveFlag_NotPresent(t *testing.T) {
	msg := &Message{
		Flags: []imap.Flag{imap.FlagSeen},
	}

	// Should be a no-op
	msg.RemoveFlag(imap.FlagDeleted)
	if len(msg.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(msg.Flags))
	}
}

func TestMessage_CopyFlags(t *testing.T) {
	msg := &Message{
		Flags: []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
	}

	copied := msg.CopyFlags()
	if len(copied) != 2 {
		t.Fatalf("expected 2 flags, got %d", len(copied))
	}

	// Modifying the copy should not affect the original
	copied[0] = imap.FlagDeleted
	if msg.Flags[0] != imap.FlagSeen {
		t.Fatal("modifying copied flags should not affect original")
	}
}

func TestMessage_ParseEnvelope(t *testing.T) {
	msg := &Message{
		Body: []byte("From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Test Message\r\nMessage-ID: <123@example.com>\r\n\r\nBody"),
	}

	env := msg.ParseEnvelope()

	if env.Subject != "Test Message" {
		t.Fatalf("expected subject %q, got %q", "Test Message", env.Subject)
	}
	if len(env.From) != 1 {
		t.Fatalf("expected 1 From address, got %d", len(env.From))
	}
	if env.From[0].Mailbox != "alice" {
		t.Fatalf("expected From mailbox %q, got %q", "alice", env.From[0].Mailbox)
	}
	if env.From[0].Host != "example.com" {
		t.Fatalf("expected From host %q, got %q", "example.com", env.From[0].Host)
	}
	if env.MessageID != "<123@example.com>" {
		t.Fatalf("expected MessageID %q, got %q", "<123@example.com>", env.MessageID)
	}
}

func TestMessage_ParseEnvelope_Empty(t *testing.T) {
	msg := &Message{
		Body: []byte("just body text"),
	}

	env := msg.ParseEnvelope()
	if env == nil {
		t.Fatal("expected non-nil envelope even with no headers")
	}
}

func TestMessage_HeaderBytes(t *testing.T) {
	body := "From: alice@example.com\r\nSubject: Test\r\n\r\nBody content"
	msg := &Message{Body: []byte(body)}

	headers := msg.HeaderBytes()
	if !bytes.Contains(headers, []byte("From:")) {
		t.Fatal("expected From header in HeaderBytes")
	}
	if !bytes.Contains(headers, []byte("Subject:")) {
		t.Fatal("expected Subject header in HeaderBytes")
	}
	if bytes.Contains(headers, []byte("Body content")) {
		t.Fatal("body content should not be in HeaderBytes")
	}
}

func TestMessage_TextBytes(t *testing.T) {
	body := "From: alice@example.com\r\nSubject: Test\r\n\r\nBody content here"
	msg := &Message{Body: []byte(body)}

	text := msg.TextBytes()
	if string(text) != "Body content here" {
		t.Fatalf("expected %q, got %q", "Body content here", string(text))
	}
}

func TestMessage_TextBytes_NoBody(t *testing.T) {
	msg := &Message{Body: []byte("just headers")}

	text := msg.TextBytes()
	if text != nil {
		t.Fatalf("expected nil, got %q", string(text))
	}
}

func TestMessage_HeaderBytes_LF(t *testing.T) {
	body := "From: alice@example.com\nSubject: Test\n\nBody content"
	msg := &Message{Body: []byte(body)}

	headers := msg.HeaderBytes()
	if !bytes.Contains(headers, []byte("From:")) {
		t.Fatal("expected From header in HeaderBytes (LF)")
	}
}

func TestMessage_TextBytes_LF(t *testing.T) {
	body := "From: alice@example.com\nSubject: Test\n\nBody content here"
	msg := &Message{Body: []byte(body)}

	text := msg.TextBytes()
	if string(text) != "Body content here" {
		t.Fatalf("expected %q, got %q", "Body content here", string(text))
	}
}

// --- parseAddressList tests ---

func TestParseAddressList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{"single address", "alice@example.com", 1},
		{"named address", "Alice <alice@example.com>", 1},
		{"multiple addresses", "alice@example.com, bob@example.com", 2},
		{"empty string", "", 0},
		{"quoted name", `"Alice Smith" <alice@example.com>`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := parseAddressList(tt.input)
			if len(addrs) != tt.wantLen {
				t.Fatalf("expected %d addresses, got %d", tt.wantLen, len(addrs))
			}
		})
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantMailbox string
		wantHost    string
	}{
		{"simple", "alice@example.com", "", "alice", "example.com"},
		{"with name", "Alice <alice@example.com>", "Alice", "alice", "example.com"},
		{"quoted name", `"Alice Smith" <alice@example.com>`, "Alice Smith", "alice", "example.com"},
		{"no host", "localuser", "", "localuser", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := parseAddress(tt.input)
			if addr == nil {
				t.Fatal("expected non-nil address")
			}
			if addr.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", addr.Name, tt.wantName)
			}
			if addr.Mailbox != tt.wantMailbox {
				t.Errorf("Mailbox: got %q, want %q", addr.Mailbox, tt.wantMailbox)
			}
			if addr.Host != tt.wantHost {
				t.Errorf("Host: got %q, want %q", addr.Host, tt.wantHost)
			}
		})
	}
}

func TestParseAddress_Empty(t *testing.T) {
	addr := parseAddress("")
	if addr != nil {
		t.Fatal("expected nil for empty string")
	}
}

// --- Verify Session implements server.Session ---

func TestSession_ImplementsSession(t *testing.T) {
	var _ server.Session = (*Session)(nil)
}

// helper: appends a test message to a mailbox
func appendTestMessage(t *testing.T, s *Session, mailbox string, body string, flags []imap.Flag) {
	t.Helper()
	b := []byte(body)
	r := imap.LiteralReader{Reader: bytes.NewReader(b), Size: int64(len(b))}
	opts := &imap.AppendOptions{Flags: flags}
	_, err := s.Append(mailbox, r, opts)
	if err != nil {
		t.Fatalf("failed to append message: %v", err)
	}
}
