# Architecture

## Layered Design

The library is organized into layers, from lowest to highest:

1. **Root package** - Core IMAP types (flags, capabilities, message structures)
2. **wire/** - Streaming parser and encoder for the IMAP wire protocol
3. **state/** - Connection state machine with validated transitions
4. **server/** and **client/** - Protocol logic built on top of wire and state
5. **extensions/** and **middleware/** - Plugin system and command pipeline

## Package Responsibilities

### Root Package (`github.com/meszmate/imap-go`)

Core IMAP types shared by client and server: connection states, flags, capabilities, message structures, number sets, search criteria, etc.

### Wire Protocol (`wire/`)

Public streaming IMAP parser (recursive descent) and encoder (fluent API). Handles atoms, quoted strings, literals, parenthesized lists, and modified UTF-7 mailbox name encoding.

### State Machine (`state/`)

Explicit connection state machine with validated transitions, before/after hooks, and command-state checking.

### Server (`server/`)

IMAP server with extensible command dispatch. Key components:
- **Server** - Accepts connections, manages lifecycle
- **Conn** - Per-connection handling (greeting, read-parse-dispatch loop)
- **Dispatcher** - Handler registry (map-based, not switch)
- **Session** - Interface that backends implement
- **Writers** - Type-safe response writers (FetchWriter, ListWriter, etc.)
- **Tracker** - Mailbox state tracking for concurrent sessions

### Client (`client/`)

IMAP client with command pipelining. Key components:
- **Client** - Connection management, command execution
- **Reader** - Background goroutine for response processing
- **Tag generator** - Atomic counter for command tags
- **Pending commands** - Track in-flight pipelined commands
- **Disconnect propagation** - On close/EOF, all pending and continuation-waiting operations are failed quickly to avoid hangs

### Extension System (`extension/`)

Registry-based plugin system with dependency resolution (topological sort). Extensions can add commands, wrap existing handlers, require session interfaces, and advertise capabilities.

### Middleware (`middleware/`)

HTTP-style middleware pipeline for server command handlers. Built-in: logging, rate limiting, metrics, panic recovery, timeout.

### Auth (`auth/`)

Pluggable SASL authentication with client and server mechanism factories. Supports PLAIN, LOGIN, CRAM-MD5, XOAUTH2, OAUTHBEARER, EXTERNAL, ANONYMOUS.

## Design Principles

1. **Zero external dependencies** in core packages
2. **Interface-based** extensibility (Session, CommandHandler, Extension)
3. **Functional options** for configuration
4. **Context propagation** for timeouts and cancellation
5. **Structured logging** via `log/slog`
6. **Thread safety** with fine-grained locking
7. **Public wire protocol** for custom parsers/encoders
