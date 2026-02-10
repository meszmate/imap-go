# imap-go

[![CI](https://github.com/meszmate/imap-go/actions/workflows/ci.yml/badge.svg)](https://github.com/meszmate/imap-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/meszmate/imap-go.svg)](https://pkg.go.dev/github.com/meszmate/imap-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A production-grade IMAP library for Go with full client, server, extension system, and middleware support. Zero external dependencies in core.

## Features

- **IMAP4rev1 and IMAP4rev2** (RFC 3501 / RFC 9051) support
- **Client** with command pipelining, IDLE, STARTTLS, connection pooling
- **Server** with extensible command dispatch, session interface, mailbox tracking
- **50+ extensions** via a registry-based plugin system
- **Middleware pipeline** (logging, rate limiting, metrics, recovery, timeout)
- **Pluggable authentication** (PLAIN, LOGIN, CRAM-MD5, XOAUTH2, OAUTHBEARER, EXTERNAL, ANONYMOUS)
- **Public wire protocol** package (streaming parser and encoder)
- **Explicit state machine** with transition validation
- **Context and structured logging** integration (`context.Context`, `log/slog`)
- **Zero external dependencies** in core packages
- **Go 1.21+** minimum

## Installation

```bash
go get github.com/meszmate/imap-go
```

## Quick Start

### Client

```go
package main

import (
    "fmt"
    "log"

    "github.com/meszmate/imap-go/client"
)

func main() {
    c, err := client.DialTLS("imap.example.com:993", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    if err := c.Login("user@example.com", "password"); err != nil {
        log.Fatal(err)
    }

    data, err := c.Select("INBOX", nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("INBOX has %d messages\n", data.NumMessages)

    if err := c.Logout(); err != nil {
        log.Fatal(err)
    }
}
```

### Server

```go
package main

import (
    "log"

    imap "github.com/meszmate/imap-go"
    "github.com/meszmate/imap-go/server"
    "github.com/meszmate/imap-go/server/memserver"
)

func main() {
    mem := memserver.New()
    mem.AddUser("user", "password")

    srv := server.New(
        server.WithNewSession(func(conn *server.Conn) (server.Session, error) {
            return mem.NewSession(), nil
        }),
    )

    log.Println("Starting IMAP server on :143")
    if err := srv.ListenAndServe(":143"); err != nil {
        log.Fatal(err)
    }
}
```

## Architecture

```
wire/          Public wire protocol (parser + encoder)
state/         Connection state machine
server/        IMAP server with extensible dispatch
client/        IMAP client with pipelining
extension/     Extension/plugin registry
extensions/    50+ built-in extension implementations
middleware/    Server middleware pipeline
auth/          Pluggable authentication mechanisms
imaptest/      Test infrastructure (harness + mocks)
```

### Key Design Decisions

- **Registry-based extensions** instead of type assertions
- **Middleware pipeline** for server (like HTTP middleware)
- **Public wire protocol** package (not internal)
- **Explicit state machine** as a first-class component
- **Functional options** for configuration
- **Session interface** for server backends

## Extensions

52 IMAP extensions in `extensions/`. Status legend:
- **Full** = command handlers + session interface + protocol parsing
- **Session** = session interface defined, capability advertised, needs WrapHandler implementation
- **Core** = handled by server core, extension just advertises capability

### Fully implemented (command handlers + session interface)

- [x] **MOVE** (RFC 6851) — MOVE command handler
- [x] **ACL** (RFC 4314) — SETACL, DELETEACL, GETACL, LISTRIGHTS, MYRIGHTS
- [x] **QUOTA** (RFC 9208) — GETQUOTA, GETQUOTAROOT, SETQUOTA
- [x] **METADATA** (RFC 5464) — SETMETADATA, GETMETADATA
- [x] **SORT** (RFC 5256) — SORT command handler
- [x] **THREAD** (RFC 5256) — THREAD command handler
- [x] **NAMESPACE** (RFC 2342) — NAMESPACE command handler
- [x] **ID** (RFC 2971) — ID command handler
- [x] **UNSELECT** (RFC 3691) — UNSELECT command with state transition
- [x] **UNAUTHENTICATE** (RFC 8437) — UNAUTHENTICATE command
- [x] **COMPRESS** (RFC 4978) — COMPRESS command (DEFLATE)
- [x] **LANGUAGE** (RFC 5255) — LANGUAGE command
- [x] **REPLACE** (RFC 8508) — REPLACE command with APPENDUID
- [x] **URLAUTH** (RFC 4467) — GENURLAUTH, RESETKEY, URLFETCH
- [x] **FILTERS** (RFC 5466) — GETFILTER, SETFILTER
- [x] **CONVERT** (RFC 5259) — CONVERT command
- [x] **NOTIFY** (RFC 5465) — NOTIFY SET/NONE
- [x] **CATENATE** (RFC 4469) — APPEND WrapHandler with CATENATE parsing
- [x] **CONDSTORE** (RFC 7162) — FETCH/STORE/SELECT/EXAMINE WrapHandler with CHANGEDSINCE/UNCHANGEDSINCE/MODSEQ parsing
- [x] **QRESYNC** (RFC 7162) — SELECT/EXAMINE WrapHandler with QRESYNC params, VANISHED (EARLIER) responses, FETCH VANISHED modifier
- [x] **UIDPLUS** (RFC 4315) — COPY/EXPUNGE WrapHandler with CopyUIDs/ExpungeUIDs routing, COPYUID response codes
- [x] **ESEARCH** (RFC 4731) — SEARCH WrapHandler with RETURN (MIN MAX COUNT ALL SAVE) options, ESEARCH response format
- [x] **LIST-EXTENDED** (RFC 5258) — LIST WrapHandler with selection options (SUBSCRIBED, REMOTE, RECURSIVEMATCH, SPECIAL-USE) and return options (SUBSCRIBED, CHILDREN, STATUS, MYRIGHTS, SPECIAL-USE)
- [x] **LIST-STATUS** (RFC 5819) — Handled via LIST-EXTENDED RETURN (STATUS) option
- [x] **LIST-MYRIGHTS** (RFC 8440) — Handled via LIST-EXTENDED RETURN (MYRIGHTS) option
- [x] **LIST-METADATA** (RFC 9590) — Handled via LIST-EXTENDED RETURN (METADATA) option
- [x] **SPECIAL-USE** (RFC 6154) — CREATE WrapHandler with USE attribute parsing; LIST handled via LIST-EXTENDED
- [x] **BINARY** (RFC 3516) — BINARY[]/BINARY.PEEK[]/BINARY.SIZE[] fetch items, binary literal ~{N} APPEND support
- [x] **SEARCHRES** (RFC 5182) — SEARCH RETURN (SAVE) with result saving, $ reference in FETCH/STORE/COPY/MOVE sequence sets and SEARCH criteria
- [x] **PARTIAL** (RFC 9394) — SEARCH/SORT RETURN (PARTIAL offset:count) with paginated results in ESEARCH response format
- [x] **SEARCH=FUZZY** (RFC 6203) — SEARCH WrapHandler with FUZZY modifier parsing, session routing to SearchFuzzy/SearchExtended/Search
- [x] **UTF8=ACCEPT** (RFC 6855) — ENABLE WrapHandler for session notification, APPEND WrapHandler with UTF8 (~{N+}) literal parsing

### Session interface defined (needs WrapHandler for full protocol support)
- [ ] **UIDONLY** (RFC 9586) — TODO: OnEnabled + suppress sequence numbers in responses
- [ ] **MULTIAPPEND** (RFC 3502) — TODO: wrap APPEND for multi-message detection
- [ ] **ESORT** (RFC 5267) — TODO: wrap SORT for extended return options
- [ ] **CONTEXT** (RFC 5267) — TODO: wrap SEARCH/SORT for CONTEXT/UPDATE
- [ ] **MULTISEARCH** (RFC 7377) — TODO: multi-mailbox SEARCH
- [ ] **PREVIEW** (RFC 8970) — session interface for FETCH PREVIEW data
- [ ] **OBJECTID** (RFC 8474) — session interface for EMAILID/THREADID/MAILBOXID
- [ ] **SAVEDATE** (RFC 8514) — session interface for SAVEDATE in FETCH/SEARCH
- [ ] **STATUS=SIZE** (RFC 8438) — core handles SIZE in STATUS

### Core-handled (capability advertisement only)

- [x] **IDLE** (RFC 2177) — handled in `server/commands/idle.go`
- [x] **ENABLE** (RFC 5161) — handled in `server/commands/enable.go`
- [x] **SASL-IR** (RFC 4959) — handled in `server/commands/authenticate.go`
- [x] **LITERAL+** (RFC 7888) — handled in `wire/` layer
- [x] **CHILDREN** (RFC 3348) — LIST attributes set by session backend
- [x] **WITHIN** (RFC 5032) — SearchCriteria.Older/Younger already in core
- [x] **SORT=DISPLAY** (RFC 5957) — SortKey DISPLAYFROM/DISPLAYTO in core
- [x] **APPENDLIMIT** (RFC 7889) — StatusData.AppendLimit in core
- [x] **INPROGRESS** (RFC 9585) — response code only
- [x] **JMAPACCESS** (RFC 9698) — capability only
- [x] **MESSAGELIMIT** (RFC 9738) — capability only

## Middleware

```go
import "github.com/meszmate/imap-go/middleware"

srv := server.New(
    server.WithNewSession(sessionFactory),
)

// Apply middleware to all commands
chain := middleware.Chain(
    middleware.Recovery(logger),
    middleware.Logging(logger),
    middleware.Timeout(30 * time.Second),
    middleware.RateLimit(100, 10),
)
```

## Authentication

```go
import (
    "github.com/meszmate/imap-go/auth"
    "github.com/meszmate/imap-go/auth/plain"
    "github.com/meszmate/imap-go/auth/xoauth2"
)

// Client-side
mechanism := plain.NewClient("user", "password")
err := c.Authenticate(mechanism)

// Register mechanisms
auth.DefaultRegistry.RegisterClient("PLAIN", plain.NewClientFactory())
auth.DefaultRegistry.RegisterServer("PLAIN", plain.NewServerFactory(authenticator))
```

## Testing

```go
import (
    "testing"
    "github.com/meszmate/imap-go/server"
    "github.com/meszmate/imap-go/server/memserver"
    "github.com/meszmate/imap-go/imaptest"
)

func TestMyFeature(t *testing.T) {
    mem := memserver.New()
    mem.AddUser("user", "pass")

    srv := server.New(
        server.WithNewSession(func(conn *server.Conn) (server.Session, error) {
            return mem.NewSession(), nil
        }),
    )

    h := imaptest.NewHarness(t, srv)
    c := h.Dial()

    if err := c.Login("user", "pass"); err != nil {
        t.Fatal(err)
    }
    // ... test your feature
}
```

## License

MIT - see [LICENSE](LICENSE).
