# IMAP Protocol Overview

## What is IMAP?

IMAP (Internet Message Access Protocol) is a protocol for accessing email messages stored on a mail server. Unlike POP3, IMAP allows clients to manage messages on the server without downloading them.

## Connection States

An IMAP connection progresses through four states:

1. **Not Authenticated** - Initial state after connection. The client must authenticate.
2. **Authenticated** - The client has logged in but no mailbox is selected.
3. **Selected** - A mailbox is open and the client can access messages.
4. **Logout** - The connection is being closed.

## Command Structure

IMAP commands follow the format:

```
tag command [arguments]\r\n
```

- **tag** - A unique identifier (e.g., `A001`, `A002`) that correlates commands with responses
- **command** - The IMAP command name (e.g., `LOGIN`, `SELECT`, `FETCH`)
- **arguments** - Command-specific arguments

## Response Types

- **Tagged response** - Completes a command: `tag OK/NO/BAD [text]`
- **Untagged response** - Server data: `* response-data`
- **Continuation request** - Server needs more data: `+ [text]`

## Data Types

- **Atom** - A string of non-special characters
- **Quoted string** - A string enclosed in double quotes
- **Literal** - Binary data preceded by `{size}\r\n`
- **Number** - A 32-bit or 64-bit unsigned integer
- **Parenthesized list** - Items enclosed in parentheses

## Message Identification

- **Sequence numbers** - Temporary numbers (1-based) assigned to messages in a mailbox
- **UIDs** - Unique identifiers that persist across sessions
- **UID validity** - A value that changes when UIDs are reassigned

## Key RFCs

- RFC 3501 - IMAP4rev1
- RFC 9051 - IMAP4rev2
