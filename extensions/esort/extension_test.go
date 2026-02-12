package esort

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// sortMockSession implements server.Session and server.SessionSort.
type sortMockSession struct {
	sortCalled   bool
	sortKind     server.NumKind
	sortCriteria []imap.SortCriterion
	sortSearch   *imap.SearchCriteria
	sortOpts     *imap.SearchOptions
	sortResult   *imap.SortData
	sortErr      error
}

func (m *sortMockSession) Sort(kind server.NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SortData, error) {
	m.sortCalled = true
	m.sortKind = kind
	m.sortCriteria = criteria
	m.sortSearch = searchCriteria
	m.sortOpts = options
	if m.sortResult != nil {
		return m.sortResult, m.sortErr
	}
	return &imap.SortData{}, m.sortErr
}

// Implement server.Session interface
func (m *sortMockSession) Close() error                                           { return nil }
func (m *sortMockSession) Login(username, password string) error                  { return nil }
func (m *sortMockSession) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	return nil, nil
}
func (m *sortMockSession) Create(mailbox string, options *imap.CreateOptions) error { return nil }
func (m *sortMockSession) Delete(mailbox string) error                              { return nil }
func (m *sortMockSession) Rename(mailbox, newName string) error                     { return nil }
func (m *sortMockSession) Subscribe(mailbox string) error                           { return nil }
func (m *sortMockSession) Unsubscribe(mailbox string) error                         { return nil }
func (m *sortMockSession) List(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	return nil
}
func (m *sortMockSession) Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
	return &imap.StatusData{Mailbox: mailbox}, nil
}
func (m *sortMockSession) Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
	return &imap.AppendData{}, nil
}
func (m *sortMockSession) Poll(w *server.UpdateWriter, allowExpunge bool) error { return nil }
func (m *sortMockSession) Idle(w *server.UpdateWriter, stop <-chan struct{}) error {
	<-stop
	return nil
}
func (m *sortMockSession) Unselect() error                                        { return nil }
func (m *sortMockSession) Expunge(w *server.ExpungeWriter, uids *imap.UIDSet) error { return nil }
func (m *sortMockSession) Search(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	return &imap.SearchData{}, nil
}
func (m *sortMockSession) Fetch(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
	return nil
}
func (m *sortMockSession) Store(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
	return nil
}
func (m *sortMockSession) Copy(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
	return &imap.CopyData{}, nil
}

var _ server.Session = (*sortMockSession)(nil)
var _ server.SessionSort = (*sortMockSession)(nil)

// esortMockSession embeds sortMockSession and adds SortExtended.
type esortMockSession struct {
	sortMockSession
	sortExtendedCalled   bool
	sortExtendedKind     server.NumKind
	sortExtendedCriteria []imap.SortCriterion
	sortExtendedSearch   *imap.SearchCriteria
	sortExtendedOpts     *imap.SearchOptions
	sortExtendedResult   *imap.SearchData
	sortExtendedErr      error
}

func (m *esortMockSession) SortExtended(kind server.NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.sortExtendedCalled = true
	m.sortExtendedKind = kind
	m.sortExtendedCriteria = criteria
	m.sortExtendedSearch = searchCriteria
	m.sortExtendedOpts = options
	if m.sortExtendedResult != nil {
		return m.sortExtendedResult, m.sortExtendedErr
	}
	return &imap.SearchData{}, m.sortExtendedErr
}

var _ SessionESort = (*esortMockSession)(nil)

func newTestCommandContext(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	// Drain output to prevent blocking
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	return &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

func newTestCommandContextWithOutput(t *testing.T, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "ESORT" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "ESORT")
	}
	if len(ext.ExtCapabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(ext.ExtCapabilities))
	}
	if ext.ExtCapabilities[0] != imap.CapESort {
		t.Errorf("cap[0] = %q, want %q", ext.ExtCapabilities[0], imap.CapESort)
	}
	if ext.ExtCapabilities[1] != imap.CapContextSort {
		t.Errorf("cap[1] = %q, want %q", ext.ExtCapabilities[1], imap.CapContextSort)
	}
}

func TestWrapHandler_ReturnsHandler(t *testing.T) {
	ext := New()
	if ext.WrapHandler("SORT", dummyHandler) == nil {
		t.Error("WrapHandler(SORT) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"SEARCH", "FETCH", "STORE", "SELECT"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestSort_WithReturnMinMax(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 5, 42}},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN MAX) (DATE) UTF-8 UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortCalled {
		t.Fatal("Sort was not called")
	}
	if sess.sortOpts == nil {
		t.Fatal("options should not be nil")
	}
	if !sess.sortOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !sess.sortOpts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
	if sess.sortOpts.ReturnAll {
		t.Error("ReturnAll should be false")
	}
	if sess.sortOpts.ReturnCount {
		t.Error("ReturnCount should be false")
	}
}

func TestSort_WithReturnAllCount(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 5, 10}},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, "RETURN (ALL COUNT) (DATE) UTF-8 FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "ALL") {
		t.Errorf("response should contain ALL, got: %s", output)
	}
	if !strings.Contains(output, "COUNT") {
		t.Errorf("response should contain COUNT, got: %s", output)
	}
}

func TestSort_WithEmptyReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 2, 3}},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, "RETURN () (DATE) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if !sess.sortCalled {
		t.Fatal("Sort was not called")
	}
	if sess.sortOpts.ReturnMin || sess.sortOpts.ReturnMax || sess.sortOpts.ReturnAll || sess.sortOpts.ReturnCount || sess.sortOpts.ReturnSave {
		t.Error("no RETURN options should be set for RETURN ()")
	}

	output := outBuf.String()
	if !strings.Contains(output, "* SORT") {
		t.Errorf("expected traditional SORT response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH for empty RETURN, got: %s", output)
	}
}

func TestSort_WithoutReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	wrapped := ext.WrapHandler("SORT", original).(server.CommandHandlerFunc)

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})
	conn := server.NewTestConn(serverConn, nil)
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	sess := &sortMockSession{}
	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("(DATE) UTF-8 ALL")),
	}

	if err := wrapped.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Fatal("original handler should be called when no RETURN")
	}

	// Also verify the simple dummyHandler path works
	_ = h
}

func TestSort_SessionESort(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{Min: 1, Max: 10, Count: 3},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN MAX COUNT) (DATE) UTF-8 FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortExtendedCalled {
		t.Fatal("SortExtended should have been called")
	}
	if sess.sortExtendedOpts == nil {
		t.Fatal("options should not be nil")
	}
	if !sess.sortExtendedOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !sess.sortExtendedOpts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
	if !sess.sortExtendedOpts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
	// Verify sort criteria were parsed
	if len(sess.sortExtendedCriteria) != 1 {
		t.Fatalf("expected 1 sort criterion, got %d", len(sess.sortExtendedCriteria))
	}
	if sess.sortExtendedCriteria[0].Key != "DATE" {
		t.Errorf("expected sort key DATE, got %s", sess.sortExtendedCriteria[0].Key)
	}
}

func TestSort_FallbackToSort(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 5}},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN) (DATE) UTF-8 UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortCalled {
		t.Fatal("Session.Sort should be called as fallback")
	}
}

func TestUIDSort_ESearchResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 42}},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindUID,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) (DATE) UTF-8 ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "UID") {
		t.Errorf("response should contain UID flag, got: %s", output)
	}
	if !strings.Contains(output, `(TAG "A001")`) {
		t.Errorf("response should contain TAG correlator, got: %s", output)
	}
}

func TestSort_ESearchResponseFormat(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 5, 42}},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) (DATE) UTF-8 ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, `* ESEARCH (TAG "A001") MIN 1 MAX 42 COUNT 3`) {
		t.Errorf("unexpected ESEARCH response format: %s", output)
	}
}

func TestSort_NoResults(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) (DATE) UTF-8 ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, `* ESEARCH (TAG "A001")`) {
		t.Errorf("expected ESEARCH with TAG but no results, got: %s", output)
	}
	if strings.Contains(output, "MIN") || strings.Contains(output, "MAX") || strings.Contains(output, "COUNT") {
		t.Errorf("empty results should not contain result items, got: %s", output)
	}
}

func TestSort_ModSeqInResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{Min: 1, Max: 5, Count: 2, ModSeq: 99999},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SORT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) (DATE) UTF-8 ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "MODSEQ 99999") {
		t.Errorf("response should contain MODSEQ, got: %s", output)
	}
}

func TestSort_ReverseSortCriteria(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{Min: 1, Max: 10, Count: 5},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN MAX) (REVERSE DATE SUBJECT) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortExtendedCalled {
		t.Fatal("SortExtended should have been called")
	}
	if len(sess.sortExtendedCriteria) != 2 {
		t.Fatalf("expected 2 sort criteria, got %d", len(sess.sortExtendedCriteria))
	}
	if !sess.sortExtendedCriteria[0].Reverse {
		t.Error("first criterion should be REVERSE")
	}
	if sess.sortExtendedCriteria[0].Key != "DATE" {
		t.Errorf("first criterion key = %q, want DATE", sess.sortExtendedCriteria[0].Key)
	}
	if sess.sortExtendedCriteria[1].Key != "SUBJECT" {
		t.Errorf("second criterion key = %q, want SUBJECT", sess.sortExtendedCriteria[1].Key)
	}
}

func TestSort_SearchCriteriaParsed(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{Count: 1},
	}
	ctx := newTestCommandContext(t, "RETURN (COUNT) (DATE) UTF-8 UNSEEN FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.sortExtendedSearch == nil {
		t.Fatal("search criteria should not be nil")
	}
	hasUnseen := false
	for _, f := range sess.sortExtendedSearch.NotFlag {
		if f == imap.FlagSeen {
			hasUnseen = true
		}
	}
	if !hasUnseen {
		t.Error("expected UNSEEN (NotFlag \\Seen)")
	}
	hasFlagged := false
	for _, f := range sess.sortExtendedSearch.Flag {
		if f == imap.FlagFlagged {
			hasFlagged = true
		}
	}
	if !hasFlagged {
		t.Error("expected FLAGGED (Flag \\Flagged)")
	}
}

func TestSort_ReturnSave(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &sortMockSession{
		sortResult: &imap.SortData{AllNums: []uint32{1, 5}},
	}
	ctx := newTestCommandContext(t, "RETURN (SAVE MIN MAX COUNT) (DATE) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortCalled {
		t.Fatal("Sort was not called")
	}
	if !sess.sortOpts.ReturnSave {
		t.Error("ReturnSave should be true")
	}
	if !sess.sortOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
}
