# Quick Start Guide

## Installation

```bash
go get github.com/meszmate/imap-go
```

## Client Usage

### Connect and Login

```go
import "github.com/meszmate/imap-go/client"

// Connect with TLS
c, err := client.DialTLS("imap.example.com:993", nil)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

// Login
if err := c.Login("user@example.com", "password"); err != nil {
    log.Fatal(err)
}
```

### List Mailboxes

```go
mailboxes, err := c.ListMailboxes("", "*")
if err != nil {
    log.Fatal(err)
}
for _, mbox := range mailboxes {
    fmt.Printf("%s\n", mbox.Mailbox)
}
```

### Select and Fetch

```go
data, err := c.Select("INBOX", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Messages: %d\n", data.NumMessages)

// Fetch messages
responses, err := c.Fetch("1:10", "(FLAGS ENVELOPE)")
```

### IDLE

```go
idle, err := c.Idle()
if err != nil {
    log.Fatal(err)
}

done := make(chan error, 1)
go func() { done <- idle.Wait() }()

select {
case err := <-done:
    if err != nil {
        log.Printf("IDLE ended: %v", err)
    }
case <-time.After(30 * time.Minute):
    if err := idle.Done(); err != nil {
        log.Printf("IDLE stop error: %v", err)
    }
}
```

Notes:
- `idle.Wait()` has no context argument.
- If the server disconnects (or you call `c.Close()`), `idle.Wait()` returns an error promptly.
- The same disconnect behavior applies to continuation-based commands like `APPEND` and SASL `AUTHENTICATE`.

### Detect Disconnection Without IDLE

```go
select {
case <-c.Done():
    log.Printf("disconnected: %v", c.DisconnectErr())
case <-time.After(1 * time.Minute):
    // Optional heartbeat if you want active probing.
    if err := c.Noop(); err != nil {
        log.Printf("NOOP failed: %v", err)
    }
}
```

## Server Usage

### Basic Server

```go
import (
    "github.com/meszmate/imap-go/server"
    "github.com/meszmate/imap-go/server/memserver"
)

mem := memserver.New()
mem.AddUser("user", "password")

srv := server.New(
    server.WithNewSession(func(conn *server.Conn) (server.Session, error) {
        return mem.NewSession(conn)
    }),
)

srv.ListenAndServe(":143")
```

### With TLS

```go
srv.ListenAndServeTLS(":993", "cert.pem", "key.pem")
```

### Custom Session

Implement the `server.Session` interface for your backend:

```go
type MySession struct {
    // your fields
}

func (s *MySession) Login(username, password string) error {
    // authenticate against your backend
}

func (s *MySession) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
    // open a mailbox
}

// ... implement remaining Session methods
```
