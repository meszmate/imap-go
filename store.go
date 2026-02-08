package imap

// StoreAction specifies how flags should be modified.
type StoreAction int

const (
	// StoreFlagsSet replaces existing flags.
	StoreFlagsSet StoreAction = iota
	// StoreFlagsAdd adds to existing flags.
	StoreFlagsAdd
	// StoreFlagsDel removes from existing flags.
	StoreFlagsDel
)

// String returns the IMAP representation of the store action.
func (a StoreAction) String() string {
	switch a {
	case StoreFlagsSet:
		return "FLAGS"
	case StoreFlagsAdd:
		return "+FLAGS"
	case StoreFlagsDel:
		return "-FLAGS"
	default:
		return "FLAGS"
	}
}

// StoreFlags specifies the flag changes for a STORE command.
type StoreFlags struct {
	// Action specifies how to modify flags.
	Action StoreAction
	// Silent prevents the server from sending updated flags.
	Silent bool
	// Flags is the list of flags to set/add/remove.
	Flags []Flag
}

// StoreOptions contains additional STORE options.
type StoreOptions struct {
	// UnchangedSince only stores if the message's mod-sequence is <= this value (CONDSTORE).
	UnchangedSince uint64
}
