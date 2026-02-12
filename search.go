package imap

import (
	"time"
)

// SearchCriteria represents the criteria for SEARCH commands.
type SearchCriteria struct {
	SeqNum *SeqSet
	UID    *UIDSet

	// Date-based criteria
	Since      time.Time
	Before     time.Time
	SentSince  time.Time
	SentBefore time.Time
	SentOn     time.Time
	On         time.Time

	// SAVEDATE criteria (RFC 8514)
	SavedBefore time.Time
	SavedSince  time.Time
	SavedOn     time.Time

	// Header criteria
	Header []SearchCriteriaHeaderField

	// Body/text criteria
	Body []string
	Text []string

	// Size criteria
	Larger  int64
	Smaller int64

	// Flag criteria
	Flag    []Flag
	NotFlag []Flag

	// ModSeq criteria (CONDSTORE)
	ModSeq *SearchCriteriaModSeq

	// Nested criteria
	Or  [][2]SearchCriteria
	Not []SearchCriteria

	// Within extension (RFC 5032)
	Younger int64 // seconds
	Older   int64 // seconds

	// Save result (SEARCHRES, RFC 5182)
	SaveResult bool

	// Fuzzy search (RFC 6203)
	Fuzzy bool
}

// SearchCriteriaHeaderField is a header field search criterion.
type SearchCriteriaHeaderField struct {
	// Key is the header field name.
	Key string
	// Value is the string to search for.
	Value string
}

// SearchCriteriaModSeq is the MODSEQ search criterion.
type SearchCriteriaModSeq struct {
	ModSeq     uint64
	MetadataName string
	MetadataType string // "shared", "priv", "all"
}

// SearchOptions specifies options for the SEARCH command.
type SearchOptions struct {
	// ReturnMin requests the MIN result.
	ReturnMin bool
	// ReturnMax requests the MAX result.
	ReturnMax bool
	// ReturnAll requests the ALL result.
	ReturnAll bool
	// ReturnCount requests the COUNT result.
	ReturnCount bool
	// ReturnSave requests the SAVE result.
	ReturnSave bool
	// ReturnContext requests a persistent search context (RFC 5267).
	ReturnContext bool
	// ReturnUpdate requests live update notifications (RFC 5267).
	ReturnUpdate bool
	// ReturnPartial requests partial results (RFC 9394).
	ReturnPartial *SearchReturnPartial
}

// SearchReturnPartial specifies partial result options.
type SearchReturnPartial struct {
	Offset int32  // negative = end-relative (RFC 9394)
	Count  uint32
}

// SearchData represents the result of a SEARCH command.
type SearchData struct {
	// AllSeqNums contains all matching sequence numbers (non-ESEARCH).
	AllSeqNums []uint32
	// AllUIDs contains all matching UIDs (non-ESEARCH).
	AllUIDs []UID

	// ESEARCH results
	UID    bool   // true if results are UIDs
	Min    uint32 // minimum sequence number or UID
	Max    uint32 // maximum sequence number or UID
	All    *SeqSet // all matching numbers
	Count  uint32 // count of matches
	ModSeq uint64 // highest mod-sequence for matched messages

	// Partial results
	Partial *SearchPartialData

	// CONTEXT=SEARCH notifications (RFC 5267)
	AddTo      []SearchContextUpdate
	RemoveFrom []SearchContextUpdate
}

// SearchContextUpdate represents an ADDTO or REMOVEFROM notification (RFC 5267).
type SearchContextUpdate struct {
	Position uint32
	SeqSet   *SeqSet
}

// SearchPartialData contains partial search results.
type SearchPartialData struct {
	Offset int32  // negative = end-relative (RFC 9394)
	Total  uint32
	UIDs   []UID
}

// MultiSearchResult represents per-mailbox search results from MULTISEARCH (RFC 7377).
type MultiSearchResult struct {
	Mailbox     string
	UIDValidity uint32
	Data        *SearchData
}
