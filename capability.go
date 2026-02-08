package imap

import (
	"strings"
	"sync"
)

// Cap represents an IMAP capability.
type Cap string

// Standard IMAP capabilities.
const (
	// Core capabilities
	CapIMAP4rev1 Cap = "IMAP4rev1"
	CapIMAP4rev2 Cap = "IMAP4rev2"

	// Authentication capabilities
	CapAuthPlain      Cap = "AUTH=PLAIN"
	CapAuthLogin      Cap = "AUTH=LOGIN"
	CapAuthCRAMMD5    Cap = "AUTH=CRAM-MD5"
	CapAuthSCRAMSHA1  Cap = "AUTH=SCRAM-SHA-1"
	CapAuthSCRAMSHA256 Cap = "AUTH=SCRAM-SHA-256"
	CapAuthSCRAMSHA1Plus  Cap = "AUTH=SCRAM-SHA-1-PLUS"
	CapAuthSCRAMSHA256Plus Cap = "AUTH=SCRAM-SHA-256-PLUS"
	CapAuthXOAuth2    Cap = "AUTH=XOAUTH2"
	CapAuthOAuthBearer Cap = "AUTH=OAUTHBEARER"
	CapAuthExternal   Cap = "AUTH=EXTERNAL"
	CapAuthAnonymous  Cap = "AUTH=ANONYMOUS"

	// RFC 4959 - SASL Initial Response
	CapSASLIR Cap = "SASL-IR"

	// RFC 2177 - IDLE
	CapIdle Cap = "IDLE"

	// RFC 2342 - Namespace
	CapNamespace Cap = "NAMESPACE"

	// RFC 2971 - ID
	CapID Cap = "ID"

	// RFC 3348 - Children
	CapChildren Cap = "CHILDREN"

	// RFC 3501 - IMAP4rev1 (implied)
	CapStartTLS Cap = "STARTTLS"
	CapLogindisabled Cap = "LOGINDISABLED"

	// RFC 3502 - Multiappend
	CapMultiAppend Cap = "MULTIAPPEND"

	// RFC 3516 - Binary
	CapBinary Cap = "BINARY"

	// RFC 3691 - Unselect
	CapUnselect Cap = "UNSELECT"

	// RFC 4314 - ACL
	CapACL Cap = "ACL"

	// RFC 4315 - UIDPLUS
	CapUIDPlus Cap = "UIDPLUS"

	// RFC 4467 - URLAUTH
	CapURLAuth Cap = "URLAUTH"

	// RFC 4469 - Catenate
	CapCatenate Cap = "CATENATE"

	// RFC 4731 - ESEARCH
	CapESearch Cap = "ESEARCH"

	// RFC 4978 - COMPRESS=DEFLATE
	CapCompressDeflate Cap = "COMPRESS=DEFLATE"

	// RFC 5032 - WITHIN
	CapWithin Cap = "WITHIN"

	// RFC 5161 - ENABLE
	CapEnable Cap = "ENABLE"

	// RFC 5182 - SEARCHRES
	CapSearchRes Cap = "SEARCHRES"

	// RFC 5255 - LANGUAGE
	CapLanguage Cap = "LANGUAGE"

	// RFC 5256 - SORT
	CapSort Cap = "SORT"

	// RFC 5256 - THREAD
	CapThreadOrderedSubject Cap = "THREAD=ORDEREDSUBJECT"
	CapThreadReferences     Cap = "THREAD=REFERENCES"

	// RFC 5258 - LIST-EXTENDED
	CapListExtended Cap = "LIST-EXTENDED"

	// RFC 5259 - CONVERT
	CapConvert Cap = "CONVERT"

	// RFC 5267 - CONTEXT=SEARCH
	CapContextSearch Cap = "CONTEXT=SEARCH"
	// RFC 5267 - CONTEXT=SORT (also ESORT)
	CapContextSort Cap = "CONTEXT=SORT"
	CapESort       Cap = "ESORT"

	// RFC 5464 - METADATA / METADATA-SERVER
	CapMetadata       Cap = "METADATA"
	CapMetadataServer Cap = "METADATA-SERVER"

	// RFC 5465 - NOTIFY
	CapNotify Cap = "NOTIFY"

	// RFC 5466 - FILTERS
	CapFilters Cap = "FILTERS"

	// RFC 5819 - LIST-STATUS
	CapListStatus Cap = "LIST-STATUS"

	// RFC 5957 - SORT=DISPLAY
	CapSortDisplay Cap = "SORT=DISPLAY"

	// RFC 6154 - SPECIAL-USE / CREATE-SPECIAL-USE
	CapSpecialUse       Cap = "SPECIAL-USE"
	CapCreateSpecialUse Cap = "CREATE-SPECIAL-USE"

	// RFC 6203 - SEARCH=FUZZY
	CapSearchFuzzy Cap = "SEARCH=FUZZY"

	// RFC 6851 - MOVE
	CapMove Cap = "MOVE"

	// RFC 6855 - UTF8=ACCEPT / UTF8=ONLY
	CapUTF8Accept Cap = "UTF8=ACCEPT"
	CapUTF8Only   Cap = "UTF8=ONLY"

	// RFC 7162 - CONDSTORE / QRESYNC
	CapCondStore Cap = "CONDSTORE"
	CapQResync   Cap = "QRESYNC"

	// RFC 7377 - MULTISEARCH
	CapMultiSearch Cap = "MULTISEARCH"

	// RFC 7628 - OAUTHBEARER (capability form)
	CapOAuthBearer Cap = "OAUTHBEARER"

	// RFC 7888 - LITERAL+ / LITERAL-
	CapLiteralPlus  Cap = "LITERAL+"
	CapLiteralMinus Cap = "LITERAL-"

	// RFC 7889 - APPENDLIMIT
	CapAppendLimit Cap = "APPENDLIMIT"

	// RFC 8437 - UNAUTHENTICATE
	CapUnauthenticate Cap = "UNAUTHENTICATE"

	// RFC 8438 - STATUS=SIZE
	CapStatusSize Cap = "STATUS=SIZE"

	// RFC 8440 - LIST-MYRIGHTS
	CapListMyRights Cap = "LIST-MYRIGHTS"

	// RFC 8474 - OBJECTID
	CapObjectID Cap = "OBJECTID"

	// RFC 8508 - REPLACE
	CapReplace Cap = "REPLACE"

	// RFC 8514 - SAVEDATE
	CapSaveDate Cap = "SAVEDATE"

	// RFC 8970 - PREVIEW
	CapPreview Cap = "PREVIEW"

	// RFC 9051 - IMAP4rev2 specific
	CapLiteralMinusIMAP4rev2 Cap = "LITERAL-" // Same as CapLiteralMinus

	// RFC 9208 - QUOTA / QUOTA=RES-*
	CapQuota          Cap = "QUOTA"
	CapQuotaResStorage Cap = "QUOTA=RES-STORAGE"
	CapQuotaResMessage Cap = "QUOTA=RES-MESSAGE"
	CapQuotaResMailbox Cap = "QUOTA=RES-MAILBOX"
	CapQuotaResAnnotation Cap = "QUOTA=RES-ANNOTATION-STORAGE"

	// RFC 9394 - PARTIAL
	CapPartial Cap = "PARTIAL"

	// RFC 9585 - INPROGRESS
	CapInProgress Cap = "INPROGRESS"

	// RFC 9586 - UIDONLY
	CapUIDOnly Cap = "UIDONLY"

	// RFC 9590 - LIST-METADATA
	CapListMetadata Cap = "LIST-METADATA"

	// RFC 9698 - JMAPACCESS
	CapJMAPAccess Cap = "JMAPACCESS"

	// RFC 9738 - MESSAGELIMIT
	CapMessageLimit Cap = "MESSAGELIMIT"
)

// CapSet is a set of IMAP capabilities.
type CapSet struct {
	mu   sync.RWMutex
	caps map[Cap]bool
}

// NewCapSet creates a new CapSet with the given capabilities.
func NewCapSet(caps ...Cap) *CapSet {
	cs := &CapSet{
		caps: make(map[Cap]bool, len(caps)),
	}
	for _, c := range caps {
		cs.caps[c] = true
	}
	return cs
}

// Has returns true if the set contains the given capability.
func (cs *CapSet) Has(cap Cap) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.caps[cap]
}

// Add adds capabilities to the set.
func (cs *CapSet) Add(caps ...Cap) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, c := range caps {
		cs.caps[c] = true
	}
}

// Remove removes capabilities from the set.
func (cs *CapSet) Remove(caps ...Cap) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, c := range caps {
		delete(cs.caps, c)
	}
}

// All returns all capabilities in the set as a slice.
func (cs *CapSet) All() []Cap {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make([]Cap, 0, len(cs.caps))
	for c := range cs.caps {
		result = append(result, c)
	}
	return result
}

// Len returns the number of capabilities in the set.
func (cs *CapSet) Len() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.caps)
}

// String returns the capabilities as a space-separated string.
func (cs *CapSet) String() string {
	caps := cs.All()
	strs := make([]string, len(caps))
	for i, c := range caps {
		strs[i] = string(c)
	}
	return strings.Join(strs, " ")
}

// Clone returns a copy of the capability set.
func (cs *CapSet) Clone() *CapSet {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	newCS := &CapSet{
		caps: make(map[Cap]bool, len(cs.caps)),
	}
	for c := range cs.caps {
		newCS.caps[c] = true
	}
	return newCS
}

// HasAuth returns true if the set contains an AUTH= capability for the given mechanism name.
func (cs *CapSet) HasAuth(mechanism string) bool {
	return cs.Has(Cap("AUTH=" + strings.ToUpper(mechanism)))
}
