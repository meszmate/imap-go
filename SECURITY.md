# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in imap-go, please report it responsibly.

**Do not open a public issue for security vulnerabilities.**

Instead, please email security concerns to the maintainers directly. Include:

1. Description of the vulnerability
2. Steps to reproduce
3. Potential impact
4. Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | Yes                |

## Security Considerations

### TLS

- Always use TLS in production (`ListenAndServeTLS` or STARTTLS)
- The `AllowInsecureAuth` option exists for development only
- STARTTLS upgrade is supported for opportunistic encryption

### Authentication

- PLAIN and LOGIN mechanisms transmit credentials in cleartext - use only over TLS
- Prefer SCRAM-SHA-256 or OAUTHBEARER for production deployments
- CRAM-MD5 is provided for legacy compatibility but uses MD5

### Literal Handling

- Configure `MaxLiteralSize` to prevent memory exhaustion from large literals
- The default limit is 256 MiB

### Rate Limiting

- Use the rate limiting middleware to prevent brute force attacks
- Configure connection limits via `MaxConnections`
