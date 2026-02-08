package imap

// Command names for IMAP commands.
const (
	// Any state commands
	CommandCapability = "CAPABILITY"
	CommandNoop       = "NOOP"
	CommandLogout     = "LOGOUT"

	// Not authenticated state commands
	CommandStartTLS     = "STARTTLS"
	CommandAuthenticate = "AUTHENTICATE"
	CommandLogin        = "LOGIN"

	// Authenticated state commands
	CommandEnable      = "ENABLE"
	CommandSelect      = "SELECT"
	CommandExamine     = "EXAMINE"
	CommandCreate      = "CREATE"
	CommandDelete      = "DELETE"
	CommandRename      = "RENAME"
	CommandSubscribe   = "SUBSCRIBE"
	CommandUnsubscribe = "UNSUBSCRIBE"
	CommandList        = "LIST"
	CommandLsub        = "LSUB"
	CommandNamespace   = "NAMESPACE"
	CommandStatus      = "STATUS"
	CommandAppend      = "APPEND"
	CommandIdle        = "IDLE"

	// Selected state commands
	CommandClose   = "CLOSE"
	CommandUnselect = "UNSELECT"
	CommandExpunge = "EXPUNGE"
	CommandSearch  = "SEARCH"
	CommandFetch   = "FETCH"
	CommandStore   = "STORE"
	CommandCopy    = "COPY"
	CommandMove    = "MOVE"
	CommandSort    = "SORT"
	CommandThread  = "THREAD"
	CommandUID     = "UID"

	// Extension commands
	CommandCompress      = "COMPRESS"
	CommandGetQuota      = "GETQUOTA"
	CommandGetQuotaRoot  = "GETQUOTAROOT"
	CommandSetQuota      = "SETQUOTA"
	CommandSetACL        = "SETACL"
	CommandDeleteACL     = "DELETEACL"
	CommandGetACL        = "GETACL"
	CommandListRights    = "LISTRIGHTS"
	CommandMyRights      = "MYRIGHTS"
	CommandSetMetadata   = "SETMETADATA"
	CommandGetMetadata   = "GETMETADATA"
	CommandReplace       = "REPLACE"
	CommandUnauthenticate = "UNAUTHENTICATE"
	CommandNotify        = "NOTIFY"
)
