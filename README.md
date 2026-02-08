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

50+ IMAP extensions are available in `extensions/`:

| Extension | RFC | Capability |
|-----------|-----|------------|
| IDLE | 2177 | IDLE |
| NAMESPACE | 2342 | NAMESPACE |
| ID | 2971 | ID |
| CHILDREN | 3348 | CHILDREN |
| MULTIAPPEND | 3502 | MULTIAPPEND |
| BINARY | 3516 | BINARY |
| UNSELECT | 3691 | UNSELECT |
| ACL | 4314 | ACL |
| UIDPLUS | 4315 | UIDPLUS |
| URLAUTH | 4467 | URLAUTH |
| CATENATE | 4469 | CATENATE |
| ESEARCH | 4731 | ESEARCH |
| SASL-IR | 4959 | SASL-IR |
| COMPRESS | 4978 | COMPRESS=DEFLATE |
| WITHIN | 5032 | WITHIN |
| ENABLE | 5161 | ENABLE |
| SEARCHRES | 5182 | SEARCHRES |
| LANGUAGE | 5255 | LANGUAGE |
| SORT | 5256 | SORT |
| THREAD | 5256 | THREAD=* |
| LIST-EXTENDED | 5258 | LIST-EXTENDED |
| CONVERT | 5259 | CONVERT |
| CONTEXT | 5267 | CONTEXT=SEARCH, ESORT |
| METADATA | 5464 | METADATA |
| NOTIFY | 5465 | NOTIFY |
| FILTERS | 5466 | FILTERS |
| LIST-STATUS | 5819 | LIST-STATUS |
| SORT=DISPLAY | 5957 | SORT=DISPLAY |
| SPECIAL-USE | 6154 | SPECIAL-USE |
| SEARCH=FUZZY | 6203 | SEARCH=FUZZY |
| MOVE | 6851 | MOVE |
| UTF8=ACCEPT | 6855 | UTF8=ACCEPT |
| CONDSTORE | 7162 | CONDSTORE |
| QRESYNC | 7162 | QRESYNC |
| MULTISEARCH | 7377 | MULTISEARCH |
| LITERAL+ | 7888 | LITERAL+ |
| APPENDLIMIT | 7889 | APPENDLIMIT |
| UNAUTHENTICATE | 8437 | UNAUTHENTICATE |
| STATUS=SIZE | 8438 | STATUS=SIZE |
| LIST-MYRIGHTS | 8440 | LIST-MYRIGHTS |
| OBJECTID | 8474 | OBJECTID |
| REPLACE | 8508 | REPLACE |
| SAVEDATE | 8514 | SAVEDATE |
| PREVIEW | 8970 | PREVIEW |
| QUOTA | 9208 | QUOTA |
| PARTIAL | 9394 | PARTIAL |
| INPROGRESS | 9585 | INPROGRESS |
| UIDONLY | 9586 | UIDONLY |
| LIST-METADATA | 9590 | LIST-METADATA |
| JMAPACCESS | 9698 | JMAPACCESS |
| MESSAGELIMIT | 9738 | MESSAGELIMIT |

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
