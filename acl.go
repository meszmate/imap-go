package imap

// ACLRight represents an ACL right character.
type ACLRight rune

// Standard ACL rights (RFC 4314).
const (
	ACLRightLookup    ACLRight = 'l'
	ACLRightRead      ACLRight = 'r'
	ACLRightSeen      ACLRight = 's'
	ACLRightWrite     ACLRight = 'w'
	ACLRightInsert    ACLRight = 'i'
	ACLRightPost      ACLRight = 'p'
	ACLRightCreate    ACLRight = 'k'
	ACLRightCreateOld ACLRight = 'c' // obsolete, equivalent to 'k'
	ACLRightDelete    ACLRight = 'x'
	ACLRightDeleteOld ACLRight = 'd' // obsolete, equivalent to 'x' + 't'
	ACLRightExpunge   ACLRight = 't'
	ACLRightAdmin     ACLRight = 'a'
)

// ACLRights is a string of ACL right characters.
type ACLRights string

// Contains checks if the rights string contains a specific right.
func (r ACLRights) Contains(right ACLRight) bool {
	for _, c := range string(r) {
		if ACLRight(c) == right {
			return true
		}
	}
	return false
}

// ACLData represents ACL data for a mailbox.
type ACLData struct {
	// Mailbox is the mailbox name.
	Mailbox string
	// Rights maps identifiers to their rights.
	Rights map[string]ACLRights
}

// ACLListRightsData represents the result of a LISTRIGHTS command.
type ACLListRightsData struct {
	// Mailbox is the mailbox name.
	Mailbox string
	// Identifier is the identifier.
	Identifier string
	// Required are the rights always granted.
	Required ACLRights
	// Optional are groups of rights that can be granted together.
	Optional []ACLRights
}

// ACLMyRightsData represents the result of a MYRIGHTS command.
type ACLMyRightsData struct {
	// Mailbox is the mailbox name.
	Mailbox string
	// Rights are the caller's rights.
	Rights ACLRights
}
