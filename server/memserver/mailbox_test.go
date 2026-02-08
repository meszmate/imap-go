package memserver

import (
	"testing"
	"time"

	imap "github.com/meszmate/imap-go"
)

// --- NewMailbox tests ---

func TestNewMailbox(t *testing.T) {
	mbox := NewMailbox("INBOX")

	if mbox.Name != "INBOX" {
		t.Fatalf("expected name %q, got %q", "INBOX", mbox.Name)
	}
	if len(mbox.Messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(mbox.Messages))
	}
	if mbox.UIDNext != 1 {
		t.Fatalf("expected UIDNext 1, got %d", mbox.UIDNext)
	}
	if mbox.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %d", mbox.UIDValidity)
	}
	if mbox.Subscribed {
		t.Fatal("expected Subscribed false by default")
	}

	// Standard flags should be present
	expectedFlags := []imap.Flag{
		imap.FlagSeen, imap.FlagAnswered, imap.FlagFlagged,
		imap.FlagDeleted, imap.FlagDraft,
	}
	if len(mbox.Flags) != len(expectedFlags) {
		t.Fatalf("expected %d flags, got %d", len(expectedFlags), len(mbox.Flags))
	}

	// PermanentFlags should include wildcard
	if len(mbox.PermanentFlags) != 6 {
		t.Fatalf("expected 6 permanent flags, got %d", len(mbox.PermanentFlags))
	}
}

// --- Append tests ---

func TestMailbox_Append(t *testing.T) {
	mbox := NewMailbox("INBOX")

	body := []byte("From: test@example.com\r\nSubject: Test\r\n\r\nBody text")
	flags := []imap.Flag{imap.FlagSeen}
	date := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	msg := mbox.Append(body, flags, date)

	if msg.UID != 1 {
		t.Fatalf("expected UID 1, got %d", msg.UID)
	}
	if mbox.UIDNext != 2 {
		t.Fatalf("expected UIDNext 2, got %d", mbox.UIDNext)
	}
	if len(mbox.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mbox.Messages))
	}
	if msg.InternalDate != date {
		t.Fatalf("expected date %v, got %v", date, msg.InternalDate)
	}
	if msg.Size != int64(len(body)) {
		t.Fatalf("expected size %d, got %d", len(body), msg.Size)
	}
	if len(msg.Flags) != 1 || msg.Flags[0] != imap.FlagSeen {
		t.Fatalf("expected flag \\Seen, got %v", msg.Flags)
	}
}

func TestMailbox_Append_ZeroDate(t *testing.T) {
	mbox := NewMailbox("INBOX")

	body := []byte("body")
	msg := mbox.Append(body, nil, time.Time{})

	if msg.InternalDate.IsZero() {
		t.Fatal("expected non-zero date when zero date is provided")
	}
}

func TestMailbox_Append_MultipleMessages(t *testing.T) {
	mbox := NewMailbox("INBOX")

	msg1 := mbox.Append([]byte("msg1"), nil, time.Now())
	msg2 := mbox.Append([]byte("msg2"), nil, time.Now())
	msg3 := mbox.Append([]byte("msg3"), nil, time.Now())

	if msg1.UID != 1 || msg2.UID != 2 || msg3.UID != 3 {
		t.Fatalf("expected UIDs 1,2,3, got %d,%d,%d", msg1.UID, msg2.UID, msg3.UID)
	}
	if mbox.UIDNext != 4 {
		t.Fatalf("expected UIDNext 4, got %d", mbox.UIDNext)
	}
	if len(mbox.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(mbox.Messages))
	}
}

func TestMailbox_Append_CopiesBody(t *testing.T) {
	mbox := NewMailbox("INBOX")

	body := []byte("original body")
	msg := mbox.Append(body, nil, time.Now())

	// Modify the original body
	body[0] = 'X'

	// The message body should not be affected
	if msg.Body[0] == 'X' {
		t.Fatal("appended message body should be independent of original")
	}
}

func TestMailbox_Append_CopiesFlags(t *testing.T) {
	mbox := NewMailbox("INBOX")

	flags := []imap.Flag{imap.FlagSeen}
	msg := mbox.Append([]byte("body"), flags, time.Now())

	// Modify the original flags
	flags[0] = imap.FlagDeleted

	// The message flags should not be affected
	if msg.Flags[0] != imap.FlagSeen {
		t.Fatal("appended message flags should be independent of original")
	}
}

// --- Expunge tests ---

func TestMailbox_Expunge(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagDeleted}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagDeleted}, time.Now())

	expunged := mbox.Expunge(nil)

	if len(expunged) != 2 {
		t.Fatalf("expected 2 expunged, got %d: %v", len(expunged), expunged)
	}
	// First expunged is seqnum 1, second should be adjusted to seqnum 2
	// (because after removing seqnum 1, old seqnum 3 becomes seqnum 2)
	if expunged[0] != 1 {
		t.Fatalf("expected first expunged seqnum 1, got %d", expunged[0])
	}
	if expunged[1] != 2 {
		t.Fatalf("expected second expunged seqnum 2, got %d", expunged[1])
	}

	if len(mbox.Messages) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(mbox.Messages))
	}
	if string(mbox.Messages[0].Body) != "msg2" {
		t.Fatalf("expected remaining message to be msg2, got %s", string(mbox.Messages[0].Body))
	}
}

func TestMailbox_Expunge_WithUIDSet(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagDeleted}, time.Now()) // UID 1
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagDeleted}, time.Now()) // UID 2
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagDeleted}, time.Now()) // UID 3

	uidSet := &imap.UIDSet{}
	uidSet.AddNum(1, 3)

	expunged := mbox.Expunge(uidSet)

	if len(expunged) != 2 {
		t.Fatalf("expected 2 expunged, got %d: %v", len(expunged), expunged)
	}
	// Message with UID 2 should remain
	if len(mbox.Messages) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(mbox.Messages))
	}
	if mbox.Messages[0].UID != 2 {
		t.Fatalf("expected remaining message UID 2, got %d", mbox.Messages[0].UID)
	}
}

func TestMailbox_Expunge_NoDeletedMessages(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagSeen}, time.Now())

	expunged := mbox.Expunge(nil)

	if len(expunged) != 0 {
		t.Fatalf("expected 0 expunged, got %d", len(expunged))
	}
	if len(mbox.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(mbox.Messages))
	}
}

func TestMailbox_Expunge_EmptyMailbox(t *testing.T) {
	mbox := NewMailbox("INBOX")

	expunged := mbox.Expunge(nil)

	if len(expunged) != 0 {
		t.Fatalf("expected 0 expunged, got %d", len(expunged))
	}
}

// --- MessageBySeqNum tests ---

func TestMailbox_MessageBySeqNum(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	msg := mbox.MessageBySeqNum(2)
	if msg == nil {
		t.Fatal("expected message at seqnum 2")
	}
	if string(msg.Body) != "msg2" {
		t.Fatalf("expected msg2, got %s", string(msg.Body))
	}
}

func TestMailbox_MessageBySeqNum_First(t *testing.T) {
	mbox := NewMailbox("INBOX")
	mbox.Append([]byte("first"), nil, time.Now())

	msg := mbox.MessageBySeqNum(1)
	if msg == nil {
		t.Fatal("expected message at seqnum 1")
	}
	if string(msg.Body) != "first" {
		t.Fatalf("expected %q, got %q", "first", string(msg.Body))
	}
}

func TestMailbox_MessageBySeqNum_Last(t *testing.T) {
	mbox := NewMailbox("INBOX")
	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("last"), nil, time.Now())

	msg := mbox.MessageBySeqNum(2)
	if msg == nil {
		t.Fatal("expected message at seqnum 2")
	}
	if string(msg.Body) != "last" {
		t.Fatalf("expected %q, got %q", "last", string(msg.Body))
	}
}

func TestMailbox_MessageBySeqNum_OutOfRange(t *testing.T) {
	mbox := NewMailbox("INBOX")
	mbox.Append([]byte("msg1"), nil, time.Now())

	msg := mbox.MessageBySeqNum(0)
	if msg != nil {
		t.Fatal("expected nil for seqnum 0")
	}

	msg = mbox.MessageBySeqNum(2)
	if msg != nil {
		t.Fatal("expected nil for seqnum beyond range")
	}
}

func TestMailbox_MessageBySeqNum_EmptyMailbox(t *testing.T) {
	mbox := NewMailbox("INBOX")

	msg := mbox.MessageBySeqNum(1)
	if msg != nil {
		t.Fatal("expected nil for empty mailbox")
	}
}

// --- MessageByUID tests ---

func TestMailbox_MessageByUID(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	msg, seqNum := mbox.MessageByUID(2)
	if msg == nil {
		t.Fatal("expected message with UID 2")
	}
	if string(msg.Body) != "msg2" {
		t.Fatalf("expected msg2, got %s", string(msg.Body))
	}
	if seqNum != 2 {
		t.Fatalf("expected seqnum 2, got %d", seqNum)
	}
}

func TestMailbox_MessageByUID_NotFound(t *testing.T) {
	mbox := NewMailbox("INBOX")
	mbox.Append([]byte("msg1"), nil, time.Now())

	msg, seqNum := mbox.MessageByUID(99)
	if msg != nil {
		t.Fatal("expected nil for non-existent UID")
	}
	if seqNum != 0 {
		t.Fatalf("expected seqnum 0, got %d", seqNum)
	}
}

func TestMailbox_MessageByUID_AfterExpunge(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagDeleted}, time.Now()) // UID 1
	mbox.Append([]byte("msg2"), nil, time.Now())                           // UID 2
	mbox.Append([]byte("msg3"), nil, time.Now())                           // UID 3

	mbox.Expunge(nil)

	// UID 1 should be gone
	msg, _ := mbox.MessageByUID(1)
	if msg != nil {
		t.Fatal("expected nil for expunged UID 1")
	}

	// UID 2 should now be at seqnum 1
	msg, seqNum := mbox.MessageByUID(2)
	if msg == nil {
		t.Fatal("expected message with UID 2")
	}
	if seqNum != 1 {
		t.Fatalf("expected seqnum 1, got %d", seqNum)
	}
}

// --- NumMessages tests ---

func TestMailbox_NumMessages(t *testing.T) {
	mbox := NewMailbox("INBOX")

	if mbox.NumMessages() != 0 {
		t.Fatalf("expected 0, got %d", mbox.NumMessages())
	}

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())

	if mbox.NumMessages() != 2 {
		t.Fatalf("expected 2, got %d", mbox.NumMessages())
	}
}

// --- NumUnseen tests ---

func TestMailbox_NumUnseen(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())                       // unseen
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagSeen}, time.Now()) // seen
	mbox.Append([]byte("msg3"), nil, time.Now())                       // unseen

	if mbox.NumUnseen() != 2 {
		t.Fatalf("expected 2 unseen, got %d", mbox.NumUnseen())
	}
}

func TestMailbox_NumUnseen_AllSeen(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagSeen}, time.Now())

	if mbox.NumUnseen() != 0 {
		t.Fatalf("expected 0 unseen, got %d", mbox.NumUnseen())
	}
}

func TestMailbox_NumUnseen_Empty(t *testing.T) {
	mbox := NewMailbox("INBOX")

	if mbox.NumUnseen() != 0 {
		t.Fatalf("expected 0, got %d", mbox.NumUnseen())
	}
}

// --- NumRecent tests ---

func TestMailbox_NumRecent(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagRecent}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagRecent}, time.Now())

	if mbox.NumRecent() != 2 {
		t.Fatalf("expected 2 recent, got %d", mbox.NumRecent())
	}
}

// --- NumDeleted tests ---

func TestMailbox_NumDeleted(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagDeleted}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagDeleted}, time.Now())

	if mbox.NumDeleted() != 2 {
		t.Fatalf("expected 2 deleted, got %d", mbox.NumDeleted())
	}
}

// --- FirstUnseen tests ---

func TestMailbox_FirstUnseen(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now()) // First unseen at seqnum 2
	mbox.Append([]byte("msg3"), nil, time.Now())

	if mbox.FirstUnseen() != 2 {
		t.Fatalf("expected first unseen at seqnum 2, got %d", mbox.FirstUnseen())
	}
}

func TestMailbox_FirstUnseen_AllSeen(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())

	if mbox.FirstUnseen() != 0 {
		t.Fatalf("expected 0 when all messages seen, got %d", mbox.FirstUnseen())
	}
}

func TestMailbox_FirstUnseen_Empty(t *testing.T) {
	mbox := NewMailbox("INBOX")

	if mbox.FirstUnseen() != 0 {
		t.Fatalf("expected 0 for empty mailbox, got %d", mbox.FirstUnseen())
	}
}

// --- TotalSize tests ---

func TestMailbox_TotalSize(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("12345"), nil, time.Now())    // 5 bytes
	mbox.Append([]byte("1234567890"), nil, time.Now()) // 10 bytes

	if mbox.TotalSize() != 15 {
		t.Fatalf("expected total size 15, got %d", mbox.TotalSize())
	}
}

func TestMailbox_TotalSize_Empty(t *testing.T) {
	mbox := NewMailbox("INBOX")

	if mbox.TotalSize() != 0 {
		t.Fatalf("expected 0, got %d", mbox.TotalSize())
	}
}

// --- SearchMessages tests ---

func TestMailbox_SearchMessages_All(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	// Nil criteria matches all
	results := mbox.SearchMessages(imap.NumKindSeq, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestMailbox_SearchMessages_ByUID(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	results := mbox.SearchMessages(imap.NumKindUID, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// UIDs should be returned
	if results[0] != 1 || results[1] != 2 || results[2] != 3 {
		t.Fatalf("expected UIDs 1,2,3, got %v", results)
	}
}

func TestMailbox_SearchMessages_BySeqNum(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())

	results := mbox.SearchMessages(imap.NumKindSeq, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != 1 || results[1] != 2 {
		t.Fatalf("expected seqnums 1,2, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByFlag(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagSeen, imap.FlagFlagged}, time.Now())

	criteria := &imap.SearchCriteria{
		Flag: []imap.Flag{imap.FlagSeen},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0] != 1 || results[1] != 3 {
		t.Fatalf("expected seqnums 1,3, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByNotFlag(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagSeen}, time.Now())

	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 2 {
		t.Fatalf("expected seqnum 2, got %v", results)
	}
}

func TestMailbox_SearchMessages_BySize(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("12345"), nil, time.Now())          // 5 bytes
	mbox.Append([]byte("1234567890abcdef"), nil, time.Now()) // 16 bytes
	mbox.Append([]byte("123"), nil, time.Now())             // 3 bytes

	criteria := &imap.SearchCriteria{
		Larger: 4,
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0] != 1 || results[1] != 2 {
		t.Fatalf("expected seqnums 1,2, got %v", results)
	}
}

func TestMailbox_SearchMessages_BySizeSmaller(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("12345"), nil, time.Now())    // 5 bytes
	mbox.Append([]byte("1234567890"), nil, time.Now()) // 10 bytes
	mbox.Append([]byte("123"), nil, time.Now())       // 3 bytes

	criteria := &imap.SearchCriteria{
		Smaller: 5,
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 3 {
		t.Fatalf("expected seqnum 3, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByDate_Since(t *testing.T) {
	mbox := NewMailbox("INBOX")

	date1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	date3 := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	mbox.Append([]byte("msg1"), nil, date1)
	mbox.Append([]byte("msg2"), nil, date2)
	mbox.Append([]byte("msg3"), nil, date3)

	criteria := &imap.SearchCriteria{
		Since: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

func TestMailbox_SearchMessages_ByDate_Before(t *testing.T) {
	mbox := NewMailbox("INBOX")

	date1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	date3 := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	mbox.Append([]byte("msg1"), nil, date1)
	mbox.Append([]byte("msg2"), nil, date2)
	mbox.Append([]byte("msg3"), nil, date3)

	criteria := &imap.SearchCriteria{
		Before: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 1 {
		t.Fatalf("expected seqnum 1, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByDate_On(t *testing.T) {
	mbox := NewMailbox("INBOX")

	date1 := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	date2 := time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC)
	date3 := time.Date(2024, 6, 16, 0, 0, 0, 0, time.UTC)

	mbox.Append([]byte("msg1"), nil, date1)
	mbox.Append([]byte("msg2"), nil, date2)
	mbox.Append([]byte("msg3"), nil, date3)

	criteria := &imap.SearchCriteria{
		On: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

func TestMailbox_SearchMessages_BySeqNumSet(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())
	mbox.Append([]byte("msg4"), nil, time.Now())

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1, 3)

	criteria := &imap.SearchCriteria{
		SeqNum: seqSet,
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0] != 1 || results[1] != 3 {
		t.Fatalf("expected seqnums 1,3, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByUIDSet(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	uidSet := &imap.UIDSet{}
	uidSet.AddNum(2)

	criteria := &imap.SearchCriteria{
		UID: uidSet,
	}

	results := mbox.SearchMessages(imap.NumKindUID, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 2 {
		t.Fatalf("expected UID 2, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByBody(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("Subject: Test\r\n\r\nHello world"), nil, time.Now())
	mbox.Append([]byte("Subject: Test\r\n\r\nGoodbye"), nil, time.Now())

	criteria := &imap.SearchCriteria{
		Body: []string{"Hello"},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
}

func TestMailbox_SearchMessages_ByText(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("Subject: Important\r\n\r\nBody text"), nil, time.Now())
	mbox.Append([]byte("Subject: Other\r\n\r\nBody text"), nil, time.Now())

	criteria := &imap.SearchCriteria{
		Text: []string{"Important"},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 1 {
		t.Fatalf("expected seqnum 1, got %v", results)
	}
}

func TestMailbox_SearchMessages_ByHeader(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("Subject: Important\r\nFrom: alice@example.com\r\n\r\nbody"), nil, time.Now())
	mbox.Append([]byte("Subject: Other\r\nFrom: bob@example.com\r\n\r\nbody"), nil, time.Now())

	criteria := &imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{
			{Key: "Subject", Value: "Important"},
		},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 1 {
		t.Fatalf("expected seqnum 1, got %v", results)
	}
}

func TestMailbox_SearchMessages_NotCriteria(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())

	criteria := &imap.SearchCriteria{
		Not: []imap.SearchCriteria{
			{Flag: []imap.Flag{imap.FlagSeen}},
		},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0] != 2 {
		t.Fatalf("expected seqnum 2, got %v", results)
	}
}

func TestMailbox_SearchMessages_OrCriteria(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagFlagged}, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	criteria := &imap.SearchCriteria{
		Or: [][2]imap.SearchCriteria{
			{
				{Flag: []imap.Flag{imap.FlagSeen}},
				{Flag: []imap.Flag{imap.FlagFlagged}},
			},
		},
	}

	results := mbox.SearchMessages(imap.NumKindSeq, criteria)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

func TestMailbox_SearchMessages_Empty(t *testing.T) {
	mbox := NewMailbox("INBOX")

	results := mbox.SearchMessages(imap.NumKindSeq, nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// --- MatchesMessages tests ---

func TestMailbox_MatchesMessages_SeqSet(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1, 3)

	matches := mbox.MatchesMessages(seqSet, imap.NumKindSeq)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].SeqNum != 1 || matches[1].SeqNum != 3 {
		t.Fatalf("expected seqnums 1,3, got %d,%d", matches[0].SeqNum, matches[1].SeqNum)
	}
}

func TestMailbox_MatchesMessages_UIDSet(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	uidSet := &imap.UIDSet{}
	uidSet.AddNum(2)

	matches := mbox.MatchesMessages(uidSet, imap.NumKindUID)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SeqNum != 2 {
		t.Fatalf("expected seqnum 2, got %d", matches[0].SeqNum)
	}
	if matches[0].Message.UID != 2 {
		t.Fatalf("expected UID 2, got %d", matches[0].Message.UID)
	}
}

func TestMailbox_MatchesMessages_Range(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())
	mbox.Append([]byte("msg4"), nil, time.Now())

	seqSet := &imap.SeqSet{}
	seqSet.AddRange(2, 3)

	matches := mbox.MatchesMessages(seqSet, imap.NumKindSeq)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].SeqNum != 2 || matches[1].SeqNum != 3 {
		t.Fatalf("expected seqnums 2,3, got %d,%d", matches[0].SeqNum, matches[1].SeqNum)
	}
}

func TestMailbox_MatchesMessages_Star(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), nil, time.Now())
	mbox.Append([]byte("msg3"), nil, time.Now())

	// 1:* means all messages
	seqSet := &imap.SeqSet{}
	seqSet.AddRange(1, 0) // 0 means "*"

	matches := mbox.MatchesMessages(seqSet, imap.NumKindSeq)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
}

func TestMailbox_MatchesMessages_Empty(t *testing.T) {
	mbox := NewMailbox("INBOX")

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(1)

	matches := mbox.MatchesMessages(seqSet, imap.NumKindSeq)
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

// --- CopyMessageTo tests ---

func TestMailbox_CopyMessageTo(t *testing.T) {
	src := NewMailbox("INBOX")
	dest := NewMailbox("Sent")

	msg := src.Append([]byte("message body"), []imap.Flag{imap.FlagSeen}, time.Now())

	newUID := src.CopyMessageTo(msg, dest)

	if newUID != 1 {
		t.Fatalf("expected new UID 1, got %d", newUID)
	}
	if len(dest.Messages) != 1 {
		t.Fatalf("expected 1 message in dest, got %d", len(dest.Messages))
	}

	copiedMsg := dest.Messages[0]
	if copiedMsg.UID != 1 {
		t.Fatalf("expected UID 1, got %d", copiedMsg.UID)
	}
	if string(copiedMsg.Body) != "message body" {
		t.Fatalf("expected body %q, got %q", "message body", string(copiedMsg.Body))
	}
	if !copiedMsg.HasFlag(imap.FlagSeen) {
		t.Fatal("copied message should have \\Seen flag")
	}
}

func TestMailbox_CopyMessageTo_RemovesRecent(t *testing.T) {
	src := NewMailbox("INBOX")
	dest := NewMailbox("Sent")

	msg := src.Append([]byte("body"), []imap.Flag{imap.FlagRecent, imap.FlagSeen}, time.Now())

	src.CopyMessageTo(msg, dest)

	copiedMsg := dest.Messages[0]
	if copiedMsg.HasFlag(imap.FlagRecent) {
		t.Fatal("\\Recent flag should be removed from copied message")
	}
	if !copiedMsg.HasFlag(imap.FlagSeen) {
		t.Fatal("\\Seen flag should be preserved in copied message")
	}
}

func TestMailbox_CopyMessageTo_IncrementsDestUID(t *testing.T) {
	src := NewMailbox("INBOX")
	dest := NewMailbox("Sent")

	msg1 := src.Append([]byte("msg1"), nil, time.Now())
	msg2 := src.Append([]byte("msg2"), nil, time.Now())

	uid1 := src.CopyMessageTo(msg1, dest)
	uid2 := src.CopyMessageTo(msg2, dest)

	if uid1 != 1 || uid2 != 2 {
		t.Fatalf("expected UIDs 1,2, got %d,%d", uid1, uid2)
	}
	if dest.UIDNext != 3 {
		t.Fatalf("expected dest UIDNext 3, got %d", dest.UIDNext)
	}
}

// --- StatusData tests ---

func TestMailbox_StatusData(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagSeen}, time.Now())
	mbox.Append([]byte("msg3"), []imap.Flag{imap.FlagDeleted}, time.Now())

	options := &imap.StatusOptions{
		NumMessages: true,
		UIDNext:     true,
		UIDValidity: true,
		NumUnseen:   true,
		NumRecent:   true,
		NumDeleted:  true,
		Size:        true,
	}

	data := mbox.StatusData("INBOX", options)

	if data.Mailbox != "INBOX" {
		t.Fatalf("expected mailbox %q, got %q", "INBOX", data.Mailbox)
	}
	if data.NumMessages == nil || *data.NumMessages != 3 {
		t.Fatalf("expected 3 messages, got %v", data.NumMessages)
	}
	if data.UIDNext == nil || *data.UIDNext != 4 {
		t.Fatalf("expected UIDNext 4, got %v", data.UIDNext)
	}
	if data.UIDValidity == nil || *data.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %v", data.UIDValidity)
	}
	if data.NumUnseen == nil || *data.NumUnseen != 2 {
		t.Fatalf("expected 2 unseen, got %v", data.NumUnseen)
	}
	if data.NumRecent == nil || *data.NumRecent != 0 {
		t.Fatalf("expected 0 recent, got %v", data.NumRecent)
	}
	if data.NumDeleted == nil || *data.NumDeleted != 1 {
		t.Fatalf("expected 1 deleted, got %v", data.NumDeleted)
	}
	if data.Size == nil || *data.Size != mbox.TotalSize() {
		t.Fatalf("expected size %d, got %v", mbox.TotalSize(), data.Size)
	}
}

func TestMailbox_StatusData_PartialOptions(t *testing.T) {
	mbox := NewMailbox("INBOX")
	mbox.Append([]byte("msg"), nil, time.Now())

	options := &imap.StatusOptions{
		NumMessages: true,
	}

	data := mbox.StatusData("INBOX", options)

	if data.NumMessages == nil {
		t.Fatal("expected NumMessages to be set")
	}
	if data.UIDNext != nil {
		t.Fatal("expected UIDNext to be nil when not requested")
	}
	if data.UIDValidity != nil {
		t.Fatal("expected UIDValidity to be nil when not requested")
	}
}

// --- SelectData tests ---

func TestMailbox_SelectData(t *testing.T) {
	mbox := NewMailbox("INBOX")

	mbox.Append([]byte("msg1"), nil, time.Now())
	mbox.Append([]byte("msg2"), []imap.Flag{imap.FlagSeen}, time.Now())

	data := mbox.SelectData(false)

	if data.NumMessages != 2 {
		t.Fatalf("expected 2 messages, got %d", data.NumMessages)
	}
	if data.UIDNext != 3 {
		t.Fatalf("expected UIDNext 3, got %d", data.UIDNext)
	}
	if data.UIDValidity != 1 {
		t.Fatalf("expected UIDValidity 1, got %d", data.UIDValidity)
	}
	if data.FirstUnseen != 1 {
		t.Fatalf("expected FirstUnseen 1, got %d", data.FirstUnseen)
	}
	if data.ReadOnly {
		t.Fatal("expected ReadOnly false")
	}
	if len(data.Flags) != 5 {
		t.Fatalf("expected 5 flags, got %d", len(data.Flags))
	}
	if len(data.PermanentFlags) != 6 {
		t.Fatalf("expected 6 permanent flags, got %d", len(data.PermanentFlags))
	}
}

func TestMailbox_SelectData_ReadOnly(t *testing.T) {
	mbox := NewMailbox("INBOX")

	data := mbox.SelectData(true)
	if !data.ReadOnly {
		t.Fatal("expected ReadOnly true")
	}
}

// --- matchPattern tests ---

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		mailbox string
		pattern string
		delim   rune
		want    bool
	}{
		{"exact match", "INBOX", "INBOX", '/', true},
		{"no match", "INBOX", "Sent", '/', false},
		{"star matches all", "INBOX", "*", '/', true},
		{"star matches nested", "Folder/Subfolder", "*", '/', true},
		{"star matches deeper nesting", "A/B/C", "A/*", '/', true},
		{"percent matches single level", "INBOX", "%", '/', true},
		{"percent does not match nested", "Folder/Subfolder", "%", '/', false},
		{"percent at end matches partial", "Sent", "S%", '/', true},
		{"star prefix", "INBOX", "INB*", '/', true},
		{"empty pattern matches empty", "", "", '/', true},
		{"empty pattern does not match non-empty", "INBOX", "", '/', false},
		{"pattern with delimiter", "Folder/Sub", "Folder/%", '/', true},
		{"pattern with delimiter deep star", "Folder/Sub/Deep", "Folder/*", '/', true},
		{"pattern with delimiter deep percent", "Folder/Sub/Deep", "Folder/%", '/', false},
		{"all children", "Parent/Child1", "Parent/*", '/', true},
		{"direct children only", "Parent/Child1", "Parent/%", '/', true},
		{"grandchildren excluded by percent", "Parent/Child/Grand", "Parent/%", '/', false},
		{"star at beginning", "anything", "*", '/', true},
		{"percent with prefix", "Test", "Te%", '/', true},
		{"percent with suffix", "Test", "%st", '/', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.mailbox, tt.pattern, tt.delim)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q, %q) = %v, want %v",
					tt.mailbox, tt.pattern, tt.delim, got, tt.want)
			}
		})
	}
}

// --- HasChildren tests ---

func TestHasChildren(t *testing.T) {
	allNames := []string{"INBOX", "Folder", "Folder/Sub1", "Folder/Sub2", "Other"}

	tests := []struct {
		name     string
		mailbox  string
		expected bool
	}{
		{"has children", "Folder", true},
		{"no children", "INBOX", false},
		{"no children for leaf", "Other", false},
		{"child has no children", "Folder/Sub1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasChildren(tt.mailbox, allNames, '/')
			if got != tt.expected {
				t.Errorf("HasChildren(%q) = %v, want %v", tt.mailbox, got, tt.expected)
			}
		})
	}
}

// --- numSetContains tests ---

func TestNumSetContains(t *testing.T) {
	tests := []struct {
		name   string
		numSet imap.NumSet
		num    uint32
		maxNum uint32
		want   bool
	}{
		{
			name: "single number match",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 5, Stop: 5},
			}},
			num: 5, maxNum: 10, want: true,
		},
		{
			name: "single number no match",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 5, Stop: 5},
			}},
			num: 3, maxNum: 10, want: false,
		},
		{
			name: "range match",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 3, Stop: 7},
			}},
			num: 5, maxNum: 10, want: true,
		},
		{
			name: "range no match below",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 3, Stop: 7},
			}},
			num: 2, maxNum: 10, want: false,
		},
		{
			name: "range no match above",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 3, Stop: 7},
			}},
			num: 8, maxNum: 10, want: false,
		},
		{
			name: "star resolves to maxNum",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 5, Stop: 0}, // 5:*
			}},
			num: 10, maxNum: 10, want: true,
		},
		{
			name: "star resolves to maxNum - below range",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 5, Stop: 0}, // 5:*
			}},
			num: 3, maxNum: 10, want: false,
		},
		{
			name: "reversed range",
			numSet: &imap.SeqSet{Set: []imap.NumRange{
				{Start: 7, Stop: 3},
			}},
			num: 5, maxNum: 10, want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numSetContains(tt.numSet, tt.num, tt.maxNum)
			if got != tt.want {
				t.Errorf("numSetContains() = %v, want %v", got, tt.want)
			}
		})
	}
}
