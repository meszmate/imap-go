package imap

import (
	"fmt"
	"strconv"
	"strings"
)

// UID represents an IMAP unique identifier.
type UID uint32

// SeqNum represents an IMAP sequence number.
type SeqNum uint32

// NumRange represents a range of numbers (sequence or UID).
// If Start == Stop, it represents a single number.
// If Stop is 0, it represents "Start:*".
type NumRange struct {
	Start uint32
	Stop  uint32 // 0 means "*"
}

// Contains checks if a number is within this range.
func (r NumRange) Contains(num uint32) bool {
	if r.Stop == 0 {
		return num >= r.Start
	}
	start, stop := r.Start, r.Stop
	if start > stop {
		start, stop = stop, start
	}
	return num >= start && num <= stop
}

// String returns the string representation of the range.
func (r NumRange) String() string {
	if r.Start == r.Stop {
		return strconv.FormatUint(uint64(r.Start), 10)
	}
	start := strconv.FormatUint(uint64(r.Start), 10)
	var stop string
	if r.Stop == 0 {
		stop = "*"
	} else {
		stop = strconv.FormatUint(uint64(r.Stop), 10)
	}
	return start + ":" + stop
}

// NumSet is the interface implemented by SeqSet and UIDSet.
type NumSet interface {
	// String returns the IMAP representation of the number set.
	String() string
	// Dynamic returns true if the set contains "*".
	Dynamic() bool
	// Ranges returns the underlying ranges.
	Ranges() []NumRange
}

// SeqSet represents a set of sequence numbers.
type SeqSet struct {
	Set []NumRange
}

// ParseSeqSet parses a sequence set string like "1,2:5,10:*".
func ParseSeqSet(s string) (*SeqSet, error) {
	ranges, err := parseNumSet(s)
	if err != nil {
		return nil, err
	}
	return &SeqSet{Set: ranges}, nil
}

// String returns the IMAP string representation.
func (ss *SeqSet) String() string {
	return formatNumSet(ss.Set)
}

// Dynamic returns true if the set contains "*".
func (ss *SeqSet) Dynamic() bool {
	for _, r := range ss.Set {
		if r.Start == 0 || r.Stop == 0 {
			return true
		}
	}
	return false
}

// Ranges returns the underlying ranges.
func (ss *SeqSet) Ranges() []NumRange {
	return ss.Set
}

// Contains checks if a sequence number is in the set.
func (ss *SeqSet) Contains(num uint32) bool {
	for _, r := range ss.Set {
		if r.Contains(num) {
			return true
		}
	}
	return false
}

// AddNum adds a single number to the set.
func (ss *SeqSet) AddNum(nums ...uint32) {
	for _, n := range nums {
		ss.Set = append(ss.Set, NumRange{Start: n, Stop: n})
	}
}

// AddRange adds a range to the set.
func (ss *SeqSet) AddRange(start, stop uint32) {
	ss.Set = append(ss.Set, NumRange{Start: start, Stop: stop})
}

// IsEmpty returns true if the set contains no ranges.
func (ss *SeqSet) IsEmpty() bool {
	return len(ss.Set) == 0
}

// UIDSet represents a set of UIDs.
type UIDSet struct {
	Set []NumRange
}

// ParseUIDSet parses a UID set string like "1,2:5,10:*".
func ParseUIDSet(s string) (*UIDSet, error) {
	ranges, err := parseNumSet(s)
	if err != nil {
		return nil, err
	}
	return &UIDSet{Set: ranges}, nil
}

// String returns the IMAP string representation.
func (us *UIDSet) String() string {
	return formatNumSet(us.Set)
}

// Dynamic returns true if the set contains "*".
func (us *UIDSet) Dynamic() bool {
	for _, r := range us.Set {
		if r.Start == 0 || r.Stop == 0 {
			return true
		}
	}
	return false
}

// Ranges returns the underlying ranges.
func (us *UIDSet) Ranges() []NumRange {
	return us.Set
}

// Contains checks if a UID is in the set.
func (us *UIDSet) Contains(uid UID) bool {
	for _, r := range us.Set {
		if r.Contains(uint32(uid)) {
			return true
		}
	}
	return false
}

// AddNum adds a single UID to the set.
func (us *UIDSet) AddNum(uids ...UID) {
	for _, u := range uids {
		us.Set = append(us.Set, NumRange{Start: uint32(u), Stop: uint32(u)})
	}
}

// AddRange adds a range of UIDs to the set.
func (us *UIDSet) AddRange(start, stop UID) {
	us.Set = append(us.Set, NumRange{Start: uint32(start), Stop: uint32(stop)})
}

// IsEmpty returns true if the set contains no ranges.
func (us *UIDSet) IsEmpty() bool {
	return len(us.Set) == 0
}

func parseNumSet(s string) ([]NumRange, error) {
	if s == "" {
		return nil, fmt.Errorf("imap: empty number set")
	}

	parts := strings.Split(s, ",")
	ranges := make([]NumRange, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("imap: empty range in number set")
		}

		colonIdx := strings.IndexByte(part, ':')
		if colonIdx < 0 {
			// Single number
			num, err := parseSeqNum(part)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, NumRange{Start: num, Stop: num})
		} else {
			// Range
			startStr := part[:colonIdx]
			stopStr := part[colonIdx+1:]
			start, err := parseSeqNum(startStr)
			if err != nil {
				return nil, err
			}
			stop, err := parseSeqNum(stopStr)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, NumRange{Start: start, Stop: stop})
		}
	}

	return ranges, nil
}

func parseSeqNum(s string) (uint32, error) {
	if s == "*" {
		return 0, nil // 0 represents "*"
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("imap: invalid number %q: %w", s, err)
	}
	if n == 0 {
		return 0, fmt.Errorf("imap: sequence number must be non-zero")
	}
	return uint32(n), nil
}

func formatNumSet(ranges []NumRange) string {
	if len(ranges) == 0 {
		return ""
	}
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = r.String()
	}
	return strings.Join(parts, ",")
}
