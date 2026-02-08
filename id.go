package imap

// ID field names as defined in RFC 2971.
const (
	IDFieldName        = "name"
	IDFieldVersion     = "version"
	IDFieldOS          = "os"
	IDFieldOSVersion   = "os-version"
	IDFieldVendor      = "vendor"
	IDFieldSupportURL  = "support-url"
	IDFieldAddress     = "address"
	IDFieldDate        = "date"
	IDFieldCommand     = "command"
	IDFieldArguments   = "arguments"
	IDFieldEnvironment = "environment"
)

// IDData represents the key-value pairs in an ID response.
// Keys are case-insensitive. Values may be nil.
type IDData map[string]*string
