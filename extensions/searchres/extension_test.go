package searchres

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// searchResMockSession embeds mock.Session and adds SessionSearchRes methods.
type searchResMockSession struct {
	mock.Session
	savedData     *imap.SearchData
	savedResult   *imap.SeqSet
	saveErr       error
	getErr        error
	saveCalled    bool
	getCalled     bool
}

func (m *searchResMockSession) SaveSearchResult(data *imap.SearchData) error {
	m.saveCalled = true
	m.savedData = data
	return m.saveErr
}

func (m *searchResMockSession) GetSearchResult() (*imap.SeqSet, error) {
	m.getCalled = true
	return m.savedResult, m.getErr
}

var _ SessionSearchRes = (*searchResMockSession)(nil)

// searchResMoveMockSession adds SessionMove support to searchResMockSession.
type searchResMoveMockSession struct {
	searchResMockSession
	moveCalled bool
	moveNumSet imap.NumSet
	moveDest   string
	moveErr    error
}

func (m *searchResMoveMockSession) Move(w *server.MoveWriter, numSet imap.NumSet, dest string) error {
	m.moveCalled = true
	m.moveNumSet = numSet
	m.moveDest = dest
	return m.moveErr
}

var _ server.SessionMove = (*searchResMoveMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCtx(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

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

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	return &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

func newTestCtxWithOutput(t *testing.T, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
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
		Name:    "SEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "SEARCHRES" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "SEARCHRES")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapSearchRes {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "ESEARCH" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
	}
}

func TestWrapHandler_Commands(t *testing.T) {
	ext := New()
	for _, name := range []string{"SEARCH", "FETCH", "STORE", "COPY", "MOVE"} {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil, want non-nil", name)
		}
	}
	for _, name := range []string{"APPEND", "NOOP", "SELECT", "LIST"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) returned non-nil, want nil", name)
		}
	}
}

func TestSessionExtension(t *testing.T) {
	ext := New()
	se := ext.SessionExtension()
	if se == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	if _, ok := se.(*SessionSearchRes); !ok {
		t.Errorf("SessionExtension() returned %T, want *SessionSearchRes", se)
	}
}

func TestOnEnabled(t *testing.T) {
	ext := New()
	if err := ext.OnEnabled("test-conn"); err != nil {
		t.Errorf("OnEnabled() returned error: %v", err)
	}
}

func TestParseReturnOptions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin bool
		wantMax bool
		wantAll bool
		wantCnt bool
		wantSav bool
	}{
		{"all options", "(MIN MAX ALL COUNT SAVE)", true, true, true, true, true},
		{"min max", "(MIN MAX)", true, true, false, false, false},
		{"save only", "(SAVE)", false, false, false, false, true},
		{"count all", "(COUNT ALL)", false, false, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := wire.NewDecoder(strings.NewReader(tt.input))
			opts := &imap.SearchOptions{}
			if err := parseReturnOptions(dec, opts); err != nil {
				t.Fatalf("parseReturnOptions() error: %v", err)
			}
			if opts.ReturnMin != tt.wantMin {
				t.Errorf("ReturnMin = %v, want %v", opts.ReturnMin, tt.wantMin)
			}
			if opts.ReturnMax != tt.wantMax {
				t.Errorf("ReturnMax = %v, want %v", opts.ReturnMax, tt.wantMax)
			}
			if opts.ReturnAll != tt.wantAll {
				t.Errorf("ReturnAll = %v, want %v", opts.ReturnAll, tt.wantAll)
			}
			if opts.ReturnCount != tt.wantCnt {
				t.Errorf("ReturnCount = %v, want %v", opts.ReturnCount, tt.wantCnt)
			}
			if opts.ReturnSave != tt.wantSav {
				t.Errorf("ReturnSave = %v, want %v", opts.ReturnSave, tt.wantSav)
			}
		})
	}
}

func TestParseReturnOptions_Empty(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("()"))
	opts := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, opts); err != nil {
		t.Fatalf("parseReturnOptions() error: %v", err)
	}
	if opts.ReturnMin || opts.ReturnMax || opts.ReturnAll || opts.ReturnCount || opts.ReturnSave {
		t.Error("empty () should have no options set")
	}
}

func TestHasAnyReturnOption(t *testing.T) {
	if hasAnyReturnOption(&imap.SearchOptions{}) {
		t.Error("empty options should return false")
	}
	if !hasAnyReturnOption(&imap.SearchOptions{ReturnMin: true}) {
		t.Error("ReturnMin should return true")
	}
	if !hasAnyReturnOption(&imap.SearchOptions{ReturnSave: true}) {
		t.Error("ReturnSave should return true")
	}
}

func TestSearch_ReturnSave_CallsSaveSearchResult(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &searchResMockSession{}
	sess.SearchFunc = func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
		return &imap.SearchData{Min: 1, Max: 5, Count: 3}, nil
	}

	ctx := newTestCtx(t, "RETURN (SAVE MIN) UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.saveCalled {
		t.Error("SaveSearchResult should have been called")
	}
	if sess.savedData == nil {
		t.Fatal("saved data should not be nil")
	}
	if sess.savedData.Min != 1 || sess.savedData.Max != 5 {
		t.Errorf("saved data = Min:%d Max:%d, want Min:1 Max:5", sess.savedData.Min, sess.savedData.Max)
	}
}

func TestSearch_DollarInCriteria(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	savedSet, _ := imap.ParseSeqSet("1:5")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotCrit *imap.SearchCriteria
	sess.SearchFunc = func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
		gotCrit = criteria
		return &imap.SearchData{AllSeqNums: []uint32{1, 2, 3, 4, 5}}, nil
	}

	ctx := newTestCtx(t, "$ UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	if gotCrit.SeqNum == nil {
		t.Fatal("SeqNum should be set from saved result")
	}
	if gotCrit.SeqNum.String() != "1:5" {
		t.Errorf("SeqNum = %s, want 1:5", gotCrit.SeqNum.String())
	}
}

func TestSearch_DollarInCriteria_WithReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	savedSet, _ := imap.ParseSeqSet("10:20")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotCrit *imap.SearchCriteria
	sess.SearchFunc = func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
		gotCrit = criteria
		return &imap.SearchData{Min: 10, Max: 20, Count: 11}, nil
	}

	ctx := newTestCtx(t, "RETURN (MIN MAX COUNT) $", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	if gotCrit.SeqNum == nil {
		t.Fatal("SeqNum should be set from saved result")
	}
	if gotCrit.SeqNum.String() != "10:20" {
		t.Errorf("SeqNum = %s, want 10:20", gotCrit.SeqNum.String())
	}
}

func TestHandleDollarFetch_NoDollar(t *testing.T) {
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	ext := New()
	h := ext.WrapHandler("FETCH", original).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "1:5 FLAGS", sess)
	ctx.Name = "FETCH"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for non-$ input")
	}
}

func TestHandleDollarFetch_WithDollar(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("1:3")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotNumSet imap.NumSet
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		gotNumSet = numSet
		return nil
	}

	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	ctx := newTestCtx(t, "$ FLAGS", sess)
	ctx.Name = "FETCH"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotNumSet == nil {
		t.Fatal("Fetch should have been called")
	}
	if gotNumSet.String() != "1:3" {
		t.Errorf("numSet = %s, want 1:3", gotNumSet.String())
	}
}

func TestHandleDollarStore_NoDollar(t *testing.T) {
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	ext := New()
	h := ext.WrapHandler("STORE", original).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "1:5 FLAGS (\\Seen)", sess)
	ctx.Name = "STORE"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for non-$ input")
	}
}

func TestHandleDollarStore_WithDollar(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("1:3")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotNumSet imap.NumSet
	var gotFlags *imap.StoreFlags
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		gotNumSet = numSet
		gotFlags = flags
		return nil
	}

	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	ctx := newTestCtx(t, "$ +FLAGS (Seen)", sess)
	ctx.Name = "STORE"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotNumSet == nil {
		t.Fatal("Store should have been called")
	}
	if gotNumSet.String() != "1:3" {
		t.Errorf("numSet = %s, want 1:3", gotNumSet.String())
	}
	if gotFlags == nil {
		t.Fatal("flags should not be nil")
	}
	if gotFlags.Action != imap.StoreFlagsAdd {
		t.Errorf("action = %v, want StoreFlagsAdd", gotFlags.Action)
	}
}

func TestHandleDollarCopy_NoDollar(t *testing.T) {
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	ext := New()
	h := ext.WrapHandler("COPY", original).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "1:5 Trash", sess)
	ctx.Name = "COPY"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for non-$ input")
	}
}

func TestHandleDollarCopy_WithDollar(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("1:3")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotNumSet imap.NumSet
	var gotDest string
	sess.CopyFunc = func(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
		gotNumSet = numSet
		gotDest = dest
		return &imap.CopyData{}, nil
	}

	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	ctx := newTestCtx(t, "$ Trash", sess)
	ctx.Name = "COPY"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotNumSet == nil {
		t.Fatal("Copy should have been called")
	}
	if gotNumSet.String() != "1:3" {
		t.Errorf("numSet = %s, want 1:3", gotNumSet.String())
	}
	if gotDest != "Trash" {
		t.Errorf("dest = %q, want %q", gotDest, "Trash")
	}
}

func TestHandleDollarMove_NoDollar(t *testing.T) {
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	ext := New()
	h := ext.WrapHandler("MOVE", original).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "1:5 Trash", sess)
	ctx.Name = "MOVE"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for non-$ input")
	}
}

func TestHandleDollarMove_WithDollar(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("1:3")
	sess := &searchResMoveMockSession{
		searchResMockSession: searchResMockSession{
			savedResult: savedSet,
		},
	}

	ext := New()
	h := ext.WrapHandler("MOVE", dummyHandler).(server.CommandHandlerFunc)

	ctx := newTestCtx(t, "$ Archive", sess)
	ctx.Name = "MOVE"

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.moveCalled {
		t.Fatal("Move should have been called")
	}
	if sess.moveNumSet == nil {
		t.Fatal("numSet should not be nil")
	}
	if sess.moveNumSet.String() != "1:3" {
		t.Errorf("numSet = %s, want 1:3", sess.moveNumSet.String())
	}
	if sess.moveDest != "Archive" {
		t.Errorf("dest = %q, want %q", sess.moveDest, "Archive")
	}
}

func TestResolveDollar_NoSession(t *testing.T) {
	sess := &mock.Session{}
	ctx := newTestCtx(t, "", sess)

	_, err := resolveDollar(ctx)
	if err == nil {
		t.Fatal("expected error for session without SessionSearchRes")
	}
}

func TestResolveDollar_EmptyResult(t *testing.T) {
	sess := &searchResMockSession{
		savedResult: &imap.SeqSet{},
	}
	ctx := newTestCtx(t, "", sess)

	_, err := resolveDollar(ctx)
	if err == nil {
		t.Fatal("expected error for empty saved result")
	}
}

func TestResolveDollar_SeqKind(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("1:5")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	ctx := newTestCtx(t, "", sess)
	ctx.NumKind = server.NumKindSeq

	numSet, err := resolveDollar(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := numSet.(*imap.SeqSet); !ok {
		t.Errorf("expected *SeqSet, got %T", numSet)
	}
	if numSet.String() != "1:5" {
		t.Errorf("numSet = %s, want 1:5", numSet.String())
	}
}

func TestResolveDollar_UIDKind(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("10:20")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	ctx := newTestCtx(t, "", sess)
	ctx.NumKind = server.NumKindUID

	numSet, err := resolveDollar(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := numSet.(*imap.UIDSet); !ok {
		t.Errorf("expected *UIDSet, got %T", numSet)
	}
	if numSet.String() != "10:20" {
		t.Errorf("numSet = %s, want 10:20", numSet.String())
	}
}

func TestSearch_ReturnSaveWithESearchResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &searchResMockSession{}
	sess.SearchFunc = func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
		return &imap.SearchData{Min: 1, Max: 10, Count: 5}, nil
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "RETURN (SAVE MIN MAX COUNT) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "MIN 1") {
		t.Errorf("response should contain MIN, got: %s", output)
	}
	if !sess.saveCalled {
		t.Error("SaveSearchResult should have been called")
	}
}

func TestSearch_WithoutSave_DoesNotCallSave(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &searchResMockSession{}
	sess.SearchFunc = func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
		return &imap.SearchData{Min: 1, Max: 5, Count: 3}, nil
	}

	ctx := newTestCtx(t, "RETURN (MIN MAX COUNT) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.saveCalled {
		t.Error("SaveSearchResult should not have been called without SAVE option")
	}
}

func TestHandleDollarFetch_UID(t *testing.T) {
	savedSet, _ := imap.ParseSeqSet("100:200")
	sess := &searchResMockSession{
		savedResult: savedSet,
	}
	var gotNumSet imap.NumSet
	var gotOpts *imap.FetchOptions
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		gotNumSet = numSet
		gotOpts = options
		return nil
	}

	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	ctx := newTestCtx(t, "$ FLAGS", sess)
	ctx.Name = "FETCH"
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotNumSet == nil {
		t.Fatal("Fetch should have been called")
	}
	if _, ok := gotNumSet.(*imap.UIDSet); !ok {
		t.Errorf("expected *UIDSet for UID FETCH, got %T", gotNumSet)
	}
	if gotOpts == nil || !gotOpts.UID {
		t.Error("UID option should be set for UID FETCH")
	}
}

func TestIsDollarCommand(t *testing.T) {
	if isDollarCommand(nil) {
		t.Error("nil decoder should return false")
	}

	dec := wire.NewDecoder(strings.NewReader("$ stuff"))
	if !isDollarCommand(dec) {
		t.Error("$ should be detected as dollar command")
	}

	dec2 := wire.NewDecoder(strings.NewReader("1:5 stuff"))
	if isDollarCommand(dec2) {
		t.Error("1:5 should not be detected as dollar command")
	}
}
