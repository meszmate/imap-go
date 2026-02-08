package state

import (
	imap "github.com/meszmate/imap-go"
)

// DefaultTransitions returns the default RFC 9051 state transition rules.
//
// The allowed transitions are:
//   - NotAuthenticated -> Authenticated (via LOGIN/AUTHENTICATE)
//   - NotAuthenticated -> Logout (via LOGOUT)
//   - Authenticated -> Selected (via SELECT/EXAMINE)
//   - Authenticated -> Logout (via LOGOUT)
//   - Authenticated -> NotAuthenticated (via UNAUTHENTICATE)
//   - Selected -> Authenticated (via CLOSE/UNSELECT)
//   - Selected -> Selected (via SELECT/EXAMINE of another mailbox)
//   - Selected -> Logout (via LOGOUT)
func DefaultTransitions() map[imap.ConnState][]imap.ConnState {
	return map[imap.ConnState][]imap.ConnState{
		imap.ConnStateNotAuthenticated: {
			imap.ConnStateAuthenticated,
			imap.ConnStateLogout,
		},
		imap.ConnStateAuthenticated: {
			imap.ConnStateSelected,
			imap.ConnStateLogout,
			imap.ConnStateNotAuthenticated, // UNAUTHENTICATE
		},
		imap.ConnStateSelected: {
			imap.ConnStateAuthenticated,
			imap.ConnStateSelected, // re-select
			imap.ConnStateLogout,
		},
	}
}

// CommandAllowedStates returns the states in which a command is allowed
// according to RFC 9051.
func CommandAllowedStates(cmd string) []imap.ConnState {
	switch cmd {
	// Any state
	case "CAPABILITY", "NOOP", "LOGOUT":
		return []imap.ConnState{
			imap.ConnStateNotAuthenticated,
			imap.ConnStateAuthenticated,
			imap.ConnStateSelected,
		}

	// Not authenticated state
	case "STARTTLS", "AUTHENTICATE", "LOGIN":
		return []imap.ConnState{
			imap.ConnStateNotAuthenticated,
		}

	// Authenticated state
	case "ENABLE", "SELECT", "EXAMINE", "CREATE", "DELETE", "RENAME",
		"SUBSCRIBE", "UNSUBSCRIBE", "LIST", "LSUB", "NAMESPACE",
		"STATUS", "APPEND", "IDLE":
		return []imap.ConnState{
			imap.ConnStateAuthenticated,
			imap.ConnStateSelected,
		}

	// Selected state
	case "CLOSE", "UNSELECT", "EXPUNGE", "SEARCH", "FETCH", "STORE",
		"COPY", "MOVE", "SORT", "THREAD", "UID":
		return []imap.ConnState{
			imap.ConnStateSelected,
		}

	default:
		return nil
	}
}
