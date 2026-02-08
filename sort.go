package imap

// SortKey represents a sort criterion key.
type SortKey string

const (
	SortKeyArrival  SortKey = "ARRIVAL"
	SortKeyCc       SortKey = "CC"
	SortKeyDate     SortKey = "DATE"
	SortKeyFrom     SortKey = "FROM"
	SortKeySize     SortKey = "SIZE"
	SortKeySubject  SortKey = "SUBJECT"
	SortKeyTo       SortKey = "TO"
	SortKeyDisplayFrom SortKey = "DISPLAYFROM" // RFC 5957
	SortKeyDisplayTo   SortKey = "DISPLAYTO"   // RFC 5957
)

// SortCriterion represents a single sort criterion.
type SortCriterion struct {
	Key     SortKey
	Reverse bool
}

// SortOptions specifies options for the SORT command.
type SortOptions struct {
	// SearchCriteria filters messages before sorting.
	SearchCriteria *SearchCriteria
	// SortCriteria specifies the sort order.
	SortCriteria []SortCriterion
	// Charset specifies the charset for search strings.
	Charset string
}

// SortData represents the result of a SORT command.
type SortData struct {
	// AllNums contains the sorted sequence numbers or UIDs.
	AllNums []uint32
}
