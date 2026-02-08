package memserver

import (
	"fmt"
	"strings"
	"sync"
	"time"

	imap "github.com/meszmate/imap-go"
)

// Mailbox represents an in-memory IMAP mailbox.
type Mailbox struct {
	mu sync.Mutex

	Name           string
	Messages       []*Message
	Flags          []imap.Flag
	PermanentFlags []imap.Flag
	UIDNext        imap.UID
	UIDValidity    uint32
	Subscribed     bool
}

// NewMailbox creates a new empty mailbox with standard flags.
func NewMailbox(name string) *Mailbox {
	return &Mailbox{
		Name: name,
		Flags: []imap.Flag{
			imap.FlagSeen,
			imap.FlagAnswered,
			imap.FlagFlagged,
			imap.FlagDeleted,
			imap.FlagDraft,
		},
		PermanentFlags: []imap.Flag{
			imap.FlagSeen,
			imap.FlagAnswered,
			imap.FlagFlagged,
			imap.FlagDeleted,
			imap.FlagDraft,
			imap.FlagWildcard,
		},
		UIDNext:     1,
		UIDValidity: 1,
		Subscribed:  false,
	}
}

// Append adds a message to the mailbox.
// The caller must hold the mailbox lock.
func (mbox *Mailbox) Append(body []byte, flags []imap.Flag, date time.Time) *Message {
	if date.IsZero() {
		date = time.Now()
	}

	uid := mbox.UIDNext
	mbox.UIDNext++

	msgFlags := make([]imap.Flag, len(flags))
	copy(msgFlags, flags)

	msg := &Message{
		UID:          uid,
		Flags:        msgFlags,
		InternalDate: date,
		Size:         int64(len(body)),
		Body:         make([]byte, len(body)),
	}
	copy(msg.Body, body)

	mbox.Messages = append(mbox.Messages, msg)
	return msg
}

// Expunge removes all messages with the \Deleted flag.
// Returns the sequence numbers that were expunged (in descending order for safe removal).
func (mbox *Mailbox) Expunge(uidSet *imap.UIDSet) []uint32 {
	var expunged []uint32
	var remaining []*Message

	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)
		if msg.HasFlag(imap.FlagDeleted) {
			if uidSet != nil && !uidSet.Contains(msg.UID) {
				remaining = append(remaining, msg)
				continue
			}
			expunged = append(expunged, seqNum)
		} else {
			remaining = append(remaining, msg)
		}
	}

	mbox.Messages = remaining

	// Adjust sequence numbers: when expunging, we need to report the adjusted
	// sequence numbers. Since we collected them in order, the first expunged
	// message's seqnum is correct, but subsequent ones need adjustment because
	// earlier messages were already removed.
	adjusted := make([]uint32, len(expunged))
	for i, seq := range expunged {
		adjusted[i] = seq - uint32(i)
	}

	return adjusted
}

// MessageBySeqNum returns the message at the given sequence number (1-based).
func (mbox *Mailbox) MessageBySeqNum(seqNum uint32) *Message {
	idx := int(seqNum) - 1
	if idx < 0 || idx >= len(mbox.Messages) {
		return nil
	}
	return mbox.Messages[idx]
}

// MessageByUID returns the message with the given UID.
func (mbox *Mailbox) MessageByUID(uid imap.UID) (*Message, uint32) {
	for i, msg := range mbox.Messages {
		if msg.UID == uid {
			return msg, uint32(i + 1)
		}
	}
	return nil, 0
}

// NumMessages returns the number of messages in the mailbox.
func (mbox *Mailbox) NumMessages() uint32 {
	return uint32(len(mbox.Messages))
}

// NumUnseen returns the number of messages without the \Seen flag.
func (mbox *Mailbox) NumUnseen() uint32 {
	var count uint32
	for _, msg := range mbox.Messages {
		if !msg.HasFlag(imap.FlagSeen) {
			count++
		}
	}
	return count
}

// NumRecent returns the number of messages with the \Recent flag.
func (mbox *Mailbox) NumRecent() uint32 {
	var count uint32
	for _, msg := range mbox.Messages {
		if msg.HasFlag(imap.FlagRecent) {
			count++
		}
	}
	return count
}

// NumDeleted returns the number of messages with the \Deleted flag.
func (mbox *Mailbox) NumDeleted() uint32 {
	var count uint32
	for _, msg := range mbox.Messages {
		if msg.HasFlag(imap.FlagDeleted) {
			count++
		}
	}
	return count
}

// FirstUnseen returns the sequence number of the first unseen message, or 0 if all are seen.
func (mbox *Mailbox) FirstUnseen() uint32 {
	for i, msg := range mbox.Messages {
		if !msg.HasFlag(imap.FlagSeen) {
			return uint32(i + 1)
		}
	}
	return 0
}

// TotalSize returns the sum of all message sizes.
func (mbox *Mailbox) TotalSize() int64 {
	var total int64
	for _, msg := range mbox.Messages {
		total += msg.Size
	}
	return total
}

// SelectData builds and returns the SelectData for this mailbox.
func (mbox *Mailbox) SelectData(readOnly bool) *imap.SelectData {
	return &imap.SelectData{
		Flags:          mbox.Flags,
		PermanentFlags: mbox.PermanentFlags,
		NumMessages:    mbox.NumMessages(),
		NumRecent:      mbox.NumRecent(),
		UIDNext:        mbox.UIDNext,
		UIDValidity:    mbox.UIDValidity,
		FirstUnseen:    mbox.FirstUnseen(),
		ReadOnly:       readOnly,
	}
}

// StatusData builds and returns the StatusData for this mailbox.
func (mbox *Mailbox) StatusData(name string, options *imap.StatusOptions) *imap.StatusData {
	data := &imap.StatusData{
		Mailbox: name,
	}

	if options.NumMessages {
		n := mbox.NumMessages()
		data.NumMessages = &n
	}
	if options.UIDNext {
		n := uint32(mbox.UIDNext)
		data.UIDNext = &n
	}
	if options.UIDValidity {
		v := mbox.UIDValidity
		data.UIDValidity = &v
	}
	if options.NumUnseen {
		n := mbox.NumUnseen()
		data.NumUnseen = &n
	}
	if options.NumRecent {
		n := mbox.NumRecent()
		data.NumRecent = &n
	}
	if options.Size {
		s := mbox.TotalSize()
		data.Size = &s
	}
	if options.NumDeleted {
		n := mbox.NumDeleted()
		data.NumDeleted = &n
	}

	return data
}

// MatchesMessages returns messages that match the given NumSet.
// kind indicates whether the set uses sequence numbers or UIDs.
func (mbox *Mailbox) MatchesMessages(numSet imap.NumSet, kind imap.NumKind) []*matchedMessage {
	var result []*matchedMessage

	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)
		var num uint32
		switch kind {
		case imap.NumKindSeq:
			num = seqNum
		case imap.NumKindUID:
			num = uint32(msg.UID)
		}

		if numSetContains(numSet, num, uint32(len(mbox.Messages))) {
			result = append(result, &matchedMessage{
				SeqNum:  seqNum,
				Message: msg,
			})
		}
	}

	return result
}

// matchedMessage pairs a message with its sequence number.
type matchedMessage struct {
	SeqNum  uint32
	Message *Message
}

// numSetContains checks if a number is contained in a NumSet.
// maxNum is used to resolve "*" (which maps to 0 in NumRange).
func numSetContains(numSet imap.NumSet, num uint32, maxNum uint32) bool {
	for _, r := range numSet.Ranges() {
		start := r.Start
		stop := r.Stop

		// Resolve "*" (represented as 0)
		if start == 0 {
			start = maxNum
		}
		if stop == 0 {
			stop = maxNum
		}

		// Normalize range direction
		if start > stop {
			start, stop = stop, start
		}

		if num >= start && num <= stop {
			return true
		}
	}
	return false
}

// SearchMessages performs a basic search on messages in the mailbox.
func (mbox *Mailbox) SearchMessages(kind imap.NumKind, criteria *imap.SearchCriteria) []uint32 {
	var results []uint32

	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		if matchesCriteria(msg, seqNum, criteria) {
			switch kind {
			case imap.NumKindSeq:
				results = append(results, seqNum)
			case imap.NumKindUID:
				results = append(results, uint32(msg.UID))
			}
		}
	}

	return results
}

// matchesCriteria checks if a message matches the given search criteria.
func matchesCriteria(msg *Message, seqNum uint32, criteria *imap.SearchCriteria) bool {
	if criteria == nil {
		return true
	}

	// Check sequence number set
	if criteria.SeqNum != nil && !criteria.SeqNum.Contains(seqNum) {
		return false
	}

	// Check UID set
	if criteria.UID != nil && !criteria.UID.Contains(msg.UID) {
		return false
	}

	// Check flags
	for _, flag := range criteria.Flag {
		if !msg.HasFlag(flag) {
			return false
		}
	}
	for _, flag := range criteria.NotFlag {
		if msg.HasFlag(flag) {
			return false
		}
	}

	// Check date criteria (internal date)
	if !criteria.Since.IsZero() && msg.InternalDate.Before(criteria.Since) {
		return false
	}
	if !criteria.Before.IsZero() && !msg.InternalDate.Before(criteria.Before) {
		return false
	}
	if !criteria.On.IsZero() {
		msgDate := msg.InternalDate.Truncate(24 * time.Hour)
		onDate := criteria.On.Truncate(24 * time.Hour)
		if !msgDate.Equal(onDate) {
			return false
		}
	}

	// Check sent date criteria (from Date header)
	if !criteria.SentSince.IsZero() || !criteria.SentBefore.IsZero() || !criteria.SentOn.IsZero() {
		env := msg.ParseEnvelope()
		if !criteria.SentSince.IsZero() && env.Date.Before(criteria.SentSince) {
			return false
		}
		if !criteria.SentBefore.IsZero() && !env.Date.Before(criteria.SentBefore) {
			return false
		}
		if !criteria.SentOn.IsZero() {
			sentDate := env.Date.Truncate(24 * time.Hour)
			onDate := criteria.SentOn.Truncate(24 * time.Hour)
			if !sentDate.Equal(onDate) {
				return false
			}
		}
	}

	// Check size criteria
	if criteria.Larger > 0 && msg.Size <= criteria.Larger {
		return false
	}
	if criteria.Smaller > 0 && msg.Size >= criteria.Smaller {
		return false
	}

	// Check header criteria
	for _, hdr := range criteria.Header {
		headers := msg.parseHeaders()
		if headers == nil {
			return false
		}
		val := headers.Get(hdr.Key)
		if hdr.Value == "" {
			// Just check header exists
			if val == "" {
				return false
			}
		} else {
			if !strings.Contains(strings.ToLower(val), strings.ToLower(hdr.Value)) {
				return false
			}
		}
	}

	// Check body text search
	for _, text := range criteria.Body {
		bodyText := msg.TextBytes()
		if !strings.Contains(strings.ToLower(string(bodyText)), strings.ToLower(text)) {
			return false
		}
	}

	// Check full text search (headers + body)
	for _, text := range criteria.Text {
		if !strings.Contains(strings.ToLower(string(msg.Body)), strings.ToLower(text)) {
			return false
		}
	}

	// Check NOT criteria
	for _, notCrit := range criteria.Not {
		if matchesCriteria(msg, seqNum, &notCrit) {
			return false
		}
	}

	// Check OR criteria
	for _, orPair := range criteria.Or {
		if !matchesCriteria(msg, seqNum, &orPair[0]) && !matchesCriteria(msg, seqNum, &orPair[1]) {
			return false
		}
	}

	return true
}

// CopyMessageTo copies a message to the destination mailbox.
// The destination mailbox lock must be held by the caller.
func (mbox *Mailbox) CopyMessageTo(msg *Message, dest *Mailbox) imap.UID {
	flags := msg.CopyFlags()
	// Remove \Recent from copied messages
	for i, f := range flags {
		if strings.EqualFold(string(f), string(imap.FlagRecent)) {
			flags = append(flags[:i], flags[i+1:]...)
			break
		}
	}

	newMsg := dest.Append(msg.Body, flags, msg.InternalDate)
	return newMsg.UID
}

// matchPattern matches a mailbox name against an IMAP LIST pattern.
// '%' matches any character except the hierarchy delimiter.
// '*' matches any characters including the hierarchy delimiter.
func matchPattern(name, pattern string, delim rune) bool {
	return matchPatternRecursive(name, pattern, delim)
}

func matchPatternRecursive(name, pattern string, delim rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// '*' matches everything, try matching rest of pattern at each position
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if matchPatternRecursive(name[i:], pattern, delim) {
					return true
				}
			}
			return false
		case '%':
			// '%' matches any character except the delimiter
			pattern = pattern[1:]
			if len(pattern) == 0 {
				// % at end, match rest if no delimiter
				return !strings.ContainsRune(name, delim)
			}
			for i := 0; i <= len(name); i++ {
				if i > 0 && rune(name[i-1]) == delim {
					break
				}
				if matchPatternRecursive(name[i:], pattern, delim) {
					return true
				}
			}
			return false
		default:
			if len(name) == 0 {
				return false
			}
			// Case-insensitive comparison for INBOX
			pc := rune(pattern[0])
			nc := rune(name[0])
			if pc != nc {
				return false
			}
			name = name[1:]
			pattern = pattern[1:]
		}
	}
	return len(name) == 0
}

// HasChildren checks if any mailbox name in the provided list is a child of this mailbox.
func HasChildren(name string, allNames []string, delim rune) bool {
	prefix := name + string(delim)
	for _, other := range allNames {
		if strings.HasPrefix(other, prefix) {
			return true
		}
	}
	return false
}

// ErrNoSuchMailbox is returned when a mailbox doesn't exist.
var ErrNoSuchMailbox = fmt.Errorf("no such mailbox")

// ErrMailboxAlreadyExists is returned when attempting to create a mailbox that already exists.
var ErrMailboxAlreadyExists = fmt.Errorf("mailbox already exists")
