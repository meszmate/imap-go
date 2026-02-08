package client

import (
	"encoding/base64"
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	imapauth "github.com/meszmate/imap-go/auth"
)

// Login authenticates the user with a username and password.
func (c *Client) Login(username, password string) error {
	// Quote username and password
	user := quoteArg(username)
	pass := quoteArg(password)

	result, err := c.execute("LOGIN", user, pass)
	if err != nil {
		return err
	}
	if result.status != "OK" {
		return &imap.IMAPError{StatusResponse: &imap.StatusResponse{
			Type: imap.StatusResponseType(result.status),
			Code: imap.ResponseCode(result.code),
			Text: result.text,
		}}
	}

	c.mu.Lock()
	c.state = imap.ConnStateAuthenticated
	c.mu.Unlock()

	return nil
}

// Authenticate authenticates using a SASL mechanism.
func (c *Client) Authenticate(mechanism imapauth.ClientMechanism) error {
	tag := c.tags.Next()
	cmd := c.pending.Add(tag)

	// Send AUTHENTICATE command
	ir, err := mechanism.Start()
	if err != nil {
		return fmt.Errorf("SASL start: %w", err)
	}

	var line strings.Builder
	line.WriteString(tag)
	line.WriteString(" AUTHENTICATE ")
	line.WriteString(mechanism.Name())
	if ir != nil && c.HasCap("SASL-IR") {
		line.WriteByte(' ')
		line.WriteString(base64.StdEncoding.EncodeToString(ir))
	}
	line.WriteString("\r\n")

	c.encoder.RawString(line.String())
	if err := c.encoder.Flush(); err != nil {
		c.pending.Complete(tag, &commandResult{err: err})
		return err
	}

	// If we didn't send IR, wait for the first continuation and send it
	if ir != nil && !c.HasCap("SASL-IR") {
		<-c.continuationCh
		encoded := base64.StdEncoding.EncodeToString(ir)
		c.encoder.RawString(encoded + "\r\n")
		if err := c.encoder.Flush(); err != nil {
			return err
		}
	}

	// Handle challenge-response loop
	for {
		select {
		case contText := <-c.continuationCh:
			// Decode challenge
			challenge, err := base64.StdEncoding.DecodeString(contText)
			if err != nil {
				// Send cancel
				c.encoder.RawString("*\r\n")
				_ = c.encoder.Flush()
				return fmt.Errorf("decoding challenge: %w", err)
			}

			// Get response
			response, err := mechanism.Next(challenge)
			if err != nil {
				c.encoder.RawString("*\r\n")
				_ = c.encoder.Flush()
				return fmt.Errorf("SASL response: %w", err)
			}

			encoded := base64.StdEncoding.EncodeToString(response)
			c.encoder.RawString(encoded + "\r\n")
			if err := c.encoder.Flush(); err != nil {
				return err
			}

		case result := <-cmd.done:
			if result.err != nil {
				return result.err
			}
			if result.status != "OK" {
				return &imap.IMAPError{StatusResponse: &imap.StatusResponse{
					Type: imap.StatusResponseType(result.status),
					Code: imap.ResponseCode(result.code),
					Text: result.text,
				}}
			}
			c.mu.Lock()
			c.state = imap.ConnStateAuthenticated
			c.mu.Unlock()
			return nil
		}
	}
}

// Logout sends the LOGOUT command and closes the connection.
func (c *Client) Logout() error {
	err := c.executeCheck("LOGOUT")
	c.mu.Lock()
	c.state = imap.ConnStateLogout
	c.mu.Unlock()
	_ = c.Close()
	return err
}

// quoteArg quotes a string for use as an IMAP argument.
func quoteArg(s string) string {
	if s == "" {
		return `""`
	}
	// Check if quoting is needed
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == ' ' || b == '"' || b == '\\' || b == '(' || b == ')' || b == '{' || b < 0x20 || b > 0x7e {
			// Use quoted string with escaping
			var buf strings.Builder
			buf.WriteByte('"')
			for j := 0; j < len(s); j++ {
				if s[j] == '"' || s[j] == '\\' {
					buf.WriteByte('\\')
				}
				buf.WriteByte(s[j])
			}
			buf.WriteByte('"')
			return buf.String()
		}
	}
	return s
}
