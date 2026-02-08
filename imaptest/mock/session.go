// Package mock provides mock implementations for testing.
package mock

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Session is a mock implementation of server.Session.
// Each method has a corresponding Func field that can be set for testing.
type Session struct {
	CloseFunc       func() error
	LoginFunc       func(username, password string) error
	SelectFunc      func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error)
	CreateFunc      func(mailbox string, options *imap.CreateOptions) error
	DeleteFunc      func(mailbox string) error
	RenameFunc      func(mailbox, newName string) error
	SubscribeFunc   func(mailbox string) error
	UnsubscribeFunc func(mailbox string) error
	ListFunc        func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
	StatusFunc      func(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error)
	AppendFunc      func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error)
	PollFunc        func(w *server.UpdateWriter, allowExpunge bool) error
	IdleFunc        func(w *server.UpdateWriter, stop <-chan struct{}) error
	UnselectFunc    func() error
	ExpungeFunc     func(w *server.ExpungeWriter, uids *imap.UIDSet) error
	SearchFunc      func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
	FetchFunc       func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error
	StoreFunc       func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error
	CopyFunc        func(numSet imap.NumSet, dest string) (*imap.CopyData, error)
}

// Ensure Session implements server.Session.
var _ server.Session = (*Session)(nil)

func (s *Session) Close() error {
	if s.CloseFunc != nil {
		return s.CloseFunc()
	}
	return nil
}

func (s *Session) Login(username, password string) error {
	if s.LoginFunc != nil {
		return s.LoginFunc(username, password)
	}
	return imap.ErrNo("LOGIN not implemented")
}

func (s *Session) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	if s.SelectFunc != nil {
		return s.SelectFunc(mailbox, options)
	}
	return nil, imap.ErrNo("SELECT not implemented")
}

func (s *Session) Create(mailbox string, options *imap.CreateOptions) error {
	if s.CreateFunc != nil {
		return s.CreateFunc(mailbox, options)
	}
	return imap.ErrNo("CREATE not implemented")
}

func (s *Session) Delete(mailbox string) error {
	if s.DeleteFunc != nil {
		return s.DeleteFunc(mailbox)
	}
	return imap.ErrNo("DELETE not implemented")
}

func (s *Session) Rename(mailbox, newName string) error {
	if s.RenameFunc != nil {
		return s.RenameFunc(mailbox, newName)
	}
	return imap.ErrNo("RENAME not implemented")
}

func (s *Session) Subscribe(mailbox string) error {
	if s.SubscribeFunc != nil {
		return s.SubscribeFunc(mailbox)
	}
	return nil
}

func (s *Session) Unsubscribe(mailbox string) error {
	if s.UnsubscribeFunc != nil {
		return s.UnsubscribeFunc(mailbox)
	}
	return nil
}

func (s *Session) List(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	if s.ListFunc != nil {
		return s.ListFunc(w, ref, patterns, options)
	}
	return nil
}

func (s *Session) Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
	if s.StatusFunc != nil {
		return s.StatusFunc(mailbox, options)
	}
	return &imap.StatusData{Mailbox: mailbox}, nil
}

func (s *Session) Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
	if s.AppendFunc != nil {
		return s.AppendFunc(mailbox, r, options)
	}
	return &imap.AppendData{}, nil
}

func (s *Session) Poll(w *server.UpdateWriter, allowExpunge bool) error {
	if s.PollFunc != nil {
		return s.PollFunc(w, allowExpunge)
	}
	return nil
}

func (s *Session) Idle(w *server.UpdateWriter, stop <-chan struct{}) error {
	if s.IdleFunc != nil {
		return s.IdleFunc(w, stop)
	}
	<-stop
	return nil
}

func (s *Session) Unselect() error {
	if s.UnselectFunc != nil {
		return s.UnselectFunc()
	}
	return nil
}

func (s *Session) Expunge(w *server.ExpungeWriter, uids *imap.UIDSet) error {
	if s.ExpungeFunc != nil {
		return s.ExpungeFunc(w, uids)
	}
	return nil
}

func (s *Session) Search(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	if s.SearchFunc != nil {
		return s.SearchFunc(kind, criteria, options)
	}
	return &imap.SearchData{}, nil
}

func (s *Session) Fetch(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
	if s.FetchFunc != nil {
		return s.FetchFunc(w, numSet, options)
	}
	return nil
}

func (s *Session) Store(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
	if s.StoreFunc != nil {
		return s.StoreFunc(w, numSet, flags, options)
	}
	return nil
}

func (s *Session) Copy(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
	if s.CopyFunc != nil {
		return s.CopyFunc(numSet, dest)
	}
	return &imap.CopyData{}, nil
}
