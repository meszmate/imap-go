# Connection States and Transitions

## States

### Not Authenticated (`ConnStateNotAuthenticated`)

The initial state after the TCP connection is established. The server sends a greeting, and the client must authenticate using `LOGIN` or `AUTHENTICATE`.

Available commands: `CAPABILITY`, `NOOP`, `LOGOUT`, `LOGIN`, `AUTHENTICATE`, `STARTTLS`

### Authenticated (`ConnStateAuthenticated`)

The client has successfully authenticated. The client can manage mailboxes and select one for message access.

Available commands: `CAPABILITY`, `NOOP`, `LOGOUT`, `SELECT`, `EXAMINE`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, `UNSUBSCRIBE`, `LIST`, `LSUB`, `STATUS`, `APPEND`, `NAMESPACE`, `ENABLE`, `IDLE`

### Selected (`ConnStateSelected`)

A mailbox has been opened. The client can access and manipulate messages.

Available commands: All authenticated-state commands plus `CLOSE`, `UNSELECT`, `EXPUNGE`, `SEARCH`, `FETCH`, `STORE`, `COPY`, `MOVE`, `UID`

### Logout (`ConnStateLogout`)

The connection is being terminated.

## Transitions

- **Not Authenticated** -> **Authenticated**: via `LOGIN` or `AUTHENTICATE`
- **Not Authenticated** -> **Not Authenticated**: via `STARTTLS` (connection upgraded, stays in same state)
- **Authenticated** -> **Selected**: via `SELECT` or `EXAMINE`
- **Selected** -> **Authenticated**: via `CLOSE` or `UNSELECT`
- **Any state** -> **Logout**: via `LOGOUT`
