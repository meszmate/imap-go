package memserver

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
	"unsafe"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Hierarchy delimiter used for mailbox names.
const Delimiter = '/'

// Session implements server.Session for the in-memory backend.
type Session struct {
	srv              *MemServer
	userData         *UserData
	selectedMailbox  *Mailbox
	selectedReadOnly bool
}

var _ server.Session = (*Session)(nil)

// Close is called when the connection is closed.
func (s *Session) Close() error {
	s.selectedMailbox = nil
	s.userData = nil
	return nil
}

// Login authenticates the user with a username and password.
func (s *Session) Login(username, password string) error {
	s.srv.mu.RLock()
	defer s.srv.mu.RUnlock()

	expected, ok := s.srv.users[username]
	if !ok || expected != password {
		return &IMAPError{Message: "invalid credentials"}
	}

	s.userData = s.srv.userData[username]
	return nil
}

// Select opens a mailbox.
func (s *Session) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	if s.userData == nil {
		return nil, &IMAPError{Message: "not authenticated"}
	}

	mbox := s.userData.GetMailbox(mailbox)
	if mbox == nil {
		return nil, ErrNoSuchMailbox
	}

	readOnly := options != nil && options.ReadOnly

	mbox.mu.Lock()
	defer mbox.mu.Unlock()

	s.selectedMailbox = mbox
	s.selectedReadOnly = readOnly

	return mbox.SelectData(readOnly), nil
}

// Create creates a new mailbox.
func (s *Session) Create(mailbox string, options *imap.CreateOptions) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}
	return s.userData.CreateMailbox(mailbox)
}

// Delete deletes a mailbox.
func (s *Session) Delete(mailbox string) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}

	// If the deleted mailbox is currently selected, unselect it
	if s.selectedMailbox != nil && s.selectedMailbox.Name == mailbox {
		s.selectedMailbox = nil
		s.selectedReadOnly = false
	}

	return s.userData.DeleteMailbox(mailbox)
}

// Rename renames a mailbox.
func (s *Session) Rename(mailbox, newName string) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}
	return s.userData.RenameMailbox(mailbox, newName)
}

// Subscribe subscribes to a mailbox.
func (s *Session) Subscribe(mailbox string) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}

	mbox := s.userData.GetMailbox(mailbox)
	if mbox == nil {
		return ErrNoSuchMailbox
	}

	mbox.mu.Lock()
	mbox.Subscribed = true
	mbox.mu.Unlock()
	return nil
}

// Unsubscribe unsubscribes from a mailbox.
func (s *Session) Unsubscribe(mailbox string) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}

	mbox := s.userData.GetMailbox(mailbox)
	if mbox == nil {
		return ErrNoSuchMailbox
	}

	mbox.mu.Lock()
	mbox.Subscribed = false
	mbox.mu.Unlock()
	return nil
}

// List lists mailboxes matching the given patterns.
func (s *Session) List(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	if s.userData == nil {
		return &IMAPError{Message: "not authenticated"}
	}

	// Special case: empty pattern returns hierarchy delimiter info
	if len(patterns) == 1 && patterns[0] == "" {
		w.WriteList(&imap.ListData{
			Delim:   Delimiter,
			Mailbox: "",
		})
		return nil
	}

	allNames := s.userData.MailboxNames()

	s.userData.mu.RLock()
	defer s.userData.mu.RUnlock()

	for name, mbox := range s.userData.Mailboxes {
		// Check if mailbox matches any pattern
		matched := false
		for _, pattern := range patterns {
			fullPattern := ref + pattern
			if matchPattern(name, fullPattern, Delimiter) {
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		// Apply select options
		if options != nil && options.SelectSubscribed && !mbox.Subscribed {
			continue
		}

		// Build attributes
		var attrs []imap.MailboxAttr

		if options != nil && options.ReturnSubscribed && mbox.Subscribed {
			attrs = append(attrs, imap.MailboxAttrSubscribed)
		}

		if options != nil && options.ReturnChildren {
			if HasChildren(name, allNames, Delimiter) {
				attrs = append(attrs, imap.MailboxAttrHasChildren)
			} else {
				attrs = append(attrs, imap.MailboxAttrHasNoChildren)
			}
		}

		data := &imap.ListData{
			Attrs:   attrs,
			Delim:   Delimiter,
			Mailbox: name,
		}

		w.WriteList(data)
	}

	return nil
}

// Status returns the status of a mailbox.
func (s *Session) Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
	if s.userData == nil {
		return nil, &IMAPError{Message: "not authenticated"}
	}

	mbox := s.userData.GetMailbox(mailbox)
	if mbox == nil {
		return nil, ErrNoSuchMailbox
	}

	mbox.mu.Lock()
	defer mbox.mu.Unlock()

	return mbox.StatusData(mailbox, options), nil
}

// Append appends a message to a mailbox.
func (s *Session) Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
	if s.userData == nil {
		return nil, &IMAPError{Message: "not authenticated"}
	}

	mbox := s.userData.GetMailbox(mailbox)
	if mbox == nil {
		return nil, ErrNoSuchMailbox
	}

	// Read the full message body
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	var flags []imap.Flag
	var internalDate time.Time
	if options != nil {
		flags = options.Flags
		internalDate = options.InternalDate
	}

	mbox.mu.Lock()
	msg := mbox.Append(body, flags, internalDate)
	mbox.mu.Unlock()

	return &imap.AppendData{
		UIDValidity: mbox.UIDValidity,
		UID:         msg.UID,
	}, nil
}

// Poll checks for mailbox updates without blocking. No-op for memserver.
func (s *Session) Poll(w *server.UpdateWriter, allowExpunge bool) error {
	return nil
}

// Idle waits for mailbox updates until stop is closed. No-op for memserver.
func (s *Session) Idle(w *server.UpdateWriter, stop <-chan struct{}) error {
	<-stop
	return nil
}

// Unselect closes the current mailbox without expunging.
func (s *Session) Unselect() error {
	s.selectedMailbox = nil
	s.selectedReadOnly = false
	return nil
}

// Expunge permanently removes messages marked as deleted.
func (s *Session) Expunge(w *server.ExpungeWriter, uids *imap.UIDSet) error {
	if s.selectedMailbox == nil {
		return &IMAPError{Message: "no mailbox selected"}
	}

	mbox := s.selectedMailbox
	mbox.mu.Lock()
	expunged := mbox.Expunge(uids)
	mbox.mu.Unlock()

	for _, seqNum := range expunged {
		w.WriteExpunge(seqNum)
	}

	return nil
}

// Search searches for messages matching the criteria.
func (s *Session) Search(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	if s.selectedMailbox == nil {
		return nil, &IMAPError{Message: "no mailbox selected"}
	}

	mbox := s.selectedMailbox
	mbox.mu.Lock()
	results := mbox.SearchMessages(imap.NumKind(kind), criteria)
	mbox.mu.Unlock()

	data := &imap.SearchData{}

	if kind == imap.NumKindUID {
		data.AllUIDs = make([]imap.UID, len(results))
		for i, r := range results {
			data.AllUIDs[i] = imap.UID(r)
		}
	} else {
		data.AllSeqNums = results
	}

	// Handle ESEARCH return options
	if options != nil && len(results) > 0 {
		if options.ReturnCount {
			data.Count = uint32(len(results))
		}
		if options.ReturnMin {
			data.Min = results[0]
			for _, r := range results[1:] {
				if r < data.Min {
					data.Min = r
				}
			}
		}
		if options.ReturnMax {
			data.Max = results[0]
			for _, r := range results[1:] {
				if r > data.Max {
					data.Max = r
				}
			}
		}
		if options.ReturnAll {
			ss := &imap.SeqSet{}
			for _, r := range results {
				ss.AddNum(r)
			}
			data.All = ss
		}
	}

	return data, nil
}

// Fetch retrieves message data.
func (s *Session) Fetch(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
	if s.selectedMailbox == nil {
		return &IMAPError{Message: "no mailbox selected"}
	}

	mbox := s.selectedMailbox
	mbox.mu.Lock()
	defer mbox.mu.Unlock()

	// Determine kind based on the NumSet type
	kind := imap.NumKindSeq
	if _, ok := numSet.(*imap.UIDSet); ok {
		kind = imap.NumKindUID
	}

	matches := mbox.MatchesMessages(numSet, kind)

	for _, m := range matches {
		msg := m.Message
		data := &imap.FetchMessageData{
			SeqNum: m.SeqNum,
		}

		if options.UID {
			data.UID = msg.UID
		}

		if options.Flags {
			data.Flags = msg.CopyFlags()
		}

		if options.InternalDate {
			data.InternalDate = msg.InternalDate
		}

		if options.RFC822Size {
			data.RFC822Size = msg.Size
		}

		if options.Envelope {
			data.Envelope = msg.ParseEnvelope()
		}

		if len(options.BodySection) > 0 {
			data.BodySection = make(map[*imap.FetchItemBodySection]imap.SectionReader)
			for _, section := range options.BodySection {
				bodyData := s.fetchSection(msg, section)
				data.BodySection[section] = imap.SectionReader{
					Reader: bytes.NewReader(bodyData),
					Size:   int64(len(bodyData)),
				}

				// Set \Seen flag unless Peek is set
				if !section.Peek && !s.selectedReadOnly {
					msg.SetFlag(imap.FlagSeen)
				}
			}
		}

		w.WriteFetchData(data)
	}

	return nil
}

// fetchSection returns the body data for a given section specification.
func (s *Session) fetchSection(msg *Message, section *imap.FetchItemBodySection) []byte {
	var data []byte

	switch strings.ToUpper(section.Specifier) {
	case "HEADER":
		data = msg.HeaderBytes()
	case "HEADER.FIELDS":
		data = filterHeaders(msg.HeaderBytes(), section.Fields, false)
	case "HEADER.FIELDS.NOT":
		data = filterHeaders(msg.HeaderBytes(), section.Fields, true)
	case "TEXT":
		data = msg.TextBytes()
	default:
		// Empty specifier = entire message
		data = msg.Body
	}

	// Apply partial
	if section.Partial != nil {
		offset := section.Partial.Offset
		if offset >= int64(len(data)) {
			return nil
		}
		end := offset + section.Partial.Count
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		data = data[offset:end]
	}

	return data
}

// filterHeaders filters message headers to include only (or exclude) the specified fields.
func filterHeaders(headerBytes []byte, fields []string, not bool) []byte {
	var result []byte
	lines := bytes.Split(headerBytes, []byte("\r\n"))
	if len(lines) == 0 {
		lines = bytes.Split(headerBytes, []byte("\n"))
	}

	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[strings.ToLower(f)] = true
	}

	include := false
	for _, line := range lines {
		if len(line) == 0 {
			break
		}

		// Check if this is a continuation line (starts with space/tab)
		if line[0] == ' ' || line[0] == '\t' {
			if include {
				result = append(result, line...)
				result = append(result, '\r', '\n')
			}
			continue
		}

		// New header field
		colonIdx := bytes.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}

		fieldName := strings.ToLower(string(bytes.TrimSpace(line[:colonIdx])))
		inSet := fieldSet[fieldName]

		if not {
			include = !inSet
		} else {
			include = inSet
		}

		if include {
			result = append(result, line...)
			result = append(result, '\r', '\n')
		}
	}

	// Terminate with CRLF
	result = append(result, '\r', '\n')
	return result
}

// Store modifies message flags.
func (s *Session) Store(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
	if s.selectedMailbox == nil {
		return &IMAPError{Message: "no mailbox selected"}
	}
	if s.selectedReadOnly {
		return &IMAPError{Message: "mailbox is read-only"}
	}

	mbox := s.selectedMailbox
	mbox.mu.Lock()
	defer mbox.mu.Unlock()

	// Determine kind based on the NumSet type
	kind := imap.NumKindSeq
	if _, ok := numSet.(*imap.UIDSet); ok {
		kind = imap.NumKindUID
	}

	matches := mbox.MatchesMessages(numSet, kind)

	for _, m := range matches {
		msg := m.Message

		switch flags.Action {
		case imap.StoreFlagsSet:
			msg.Flags = make([]imap.Flag, len(flags.Flags))
			copy(msg.Flags, flags.Flags)
		case imap.StoreFlagsAdd:
			for _, f := range flags.Flags {
				msg.SetFlag(f)
			}
		case imap.StoreFlagsDel:
			for _, f := range flags.Flags {
				msg.RemoveFlag(f)
			}
		}

		// Send updated flags unless silent
		if !flags.Silent {
			w.WriteFlags(m.SeqNum, msg.CopyFlags())
		}
	}

	return nil
}

// Copy copies messages to another mailbox.
func (s *Session) Copy(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
	if s.selectedMailbox == nil {
		return nil, &IMAPError{Message: "no mailbox selected"}
	}

	destMbox := s.userData.GetMailbox(dest)
	if destMbox == nil {
		return nil, ErrNoSuchMailbox
	}

	srcMbox := s.selectedMailbox

	// Lock both mailboxes. To avoid deadlock, always lock in a consistent order
	// based on pointer address.
	srcPtr := uintptr(unsafe.Pointer(srcMbox))
	destPtr := uintptr(unsafe.Pointer(destMbox))

	if srcPtr < destPtr {
		srcMbox.mu.Lock()
		destMbox.mu.Lock()
	} else if srcMbox == destMbox {
		srcMbox.mu.Lock()
	} else {
		destMbox.mu.Lock()
		srcMbox.mu.Lock()
	}

	defer func() {
		srcMbox.mu.Unlock()
		if srcMbox != destMbox {
			destMbox.mu.Unlock()
		}
	}()

	// Determine kind based on the NumSet type
	kind := imap.NumKindSeq
	if _, ok := numSet.(*imap.UIDSet); ok {
		kind = imap.NumKindUID
	}

	matches := srcMbox.MatchesMessages(numSet, kind)

	copyData := &imap.CopyData{
		UIDValidity: destMbox.UIDValidity,
	}

	for _, m := range matches {
		newUID := srcMbox.CopyMessageTo(m.Message, destMbox)
		copyData.SourceUIDs.AddNum(m.Message.UID)
		copyData.DestUIDs.AddNum(newUID)
	}

	return copyData, nil
}

