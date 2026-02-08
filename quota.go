package imap

// QuotaResource represents a quota resource type.
type QuotaResource string

const (
	QuotaResourceStorage         QuotaResource = "STORAGE"
	QuotaResourceMessage         QuotaResource = "MESSAGE"
	QuotaResourceMailbox         QuotaResource = "MAILBOX"
	QuotaResourceAnnotationStorage QuotaResource = "ANNOTATION-STORAGE"
)

// QuotaResourceData contains usage and limit for a single resource.
type QuotaResourceData struct {
	Name  QuotaResource
	Usage int64
	Limit int64
}

// QuotaData represents the result of a GETQUOTA command.
type QuotaData struct {
	// Root is the quota root name.
	Root string
	// Resources lists the resource limits and usage.
	Resources []QuotaResourceData
}

// QuotaRootData represents the result of a GETQUOTAROOT command.
type QuotaRootData struct {
	// Mailbox is the mailbox name.
	Mailbox string
	// Roots is the list of quota root names.
	Roots []string
}
