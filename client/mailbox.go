package client

import (
	"fmt"
	"strconv"
	"strings"

	imap "github.com/meszmate/imap-go"
)

// Select selects a mailbox.
func (c *Client) Select(mailbox string, opts *imap.SelectOptions) (*imap.SelectData, error) {
	cmd := "SELECT"
	if opts != nil && opts.ReadOnly {
		cmd = "EXAMINE"
	}

	// Clear any previous untagged data
	c.collectUntagged()

	result, err := c.execute(cmd, quoteArg(mailbox))
	if err != nil {
		return nil, err
	}
	if result.status != "OK" {
		return nil, &imap.IMAPError{StatusResponse: &imap.StatusResponse{
			Type: imap.StatusResponseType(result.status),
			Code: imap.ResponseCode(result.code),
			Text: result.text,
		}}
	}

	c.mu.Lock()
	c.state = imap.ConnStateSelected
	c.mailboxName = mailbox
	data := &imap.SelectData{
		NumMessages: c.mailboxMessages,
		NumRecent:   c.mailboxRecent,
		UIDNext:     imap.UID(c.mailboxUIDNext),
		UIDValidity: c.mailboxUIDValidity,
		FirstUnseen: c.mailboxUnseen,
		ReadOnly:    c.mailboxReadOnly,
	}
	c.mu.Unlock()

	return data, nil
}

// Examine opens a mailbox in read-only mode.
func (c *Client) Examine(mailbox string) (*imap.SelectData, error) {
	return c.Select(mailbox, &imap.SelectOptions{ReadOnly: true})
}

// Create creates a new mailbox.
func (c *Client) Create(mailbox string) error {
	return c.executeCheck("CREATE", quoteArg(mailbox))
}

// Delete deletes a mailbox.
func (c *Client) Delete(mailbox string) error {
	return c.executeCheck("DELETE", quoteArg(mailbox))
}

// Rename renames a mailbox.
func (c *Client) Rename(oldName, newName string) error {
	return c.executeCheck("RENAME", quoteArg(oldName), quoteArg(newName))
}

// Subscribe subscribes to a mailbox.
func (c *Client) Subscribe(mailbox string) error {
	return c.executeCheck("SUBSCRIBE", quoteArg(mailbox))
}

// Unsubscribe unsubscribes from a mailbox.
func (c *Client) Unsubscribe(mailbox string) error {
	return c.executeCheck("UNSUBSCRIBE", quoteArg(mailbox))
}

// ListMailboxes lists mailboxes matching the given reference and pattern.
func (c *Client) ListMailboxes(ref, pattern string) ([]*imap.ListData, error) {
	c.collectUntagged()

	result, err := c.execute("LIST", quoteArg(ref), quoteArg(pattern))
	if err != nil {
		return nil, err
	}
	if result.status != "OK" {
		return nil, &imap.IMAPError{StatusResponse: &imap.StatusResponse{
			Type: imap.StatusResponseType(result.status),
			Code: imap.ResponseCode(result.code),
			Text: result.text,
		}}
	}

	untagged := c.collectUntagged()
	var mailboxes []*imap.ListData
	for _, line := range untagged {
		if strings.HasPrefix(line, "LIST ") {
			data := parseListResponse(line[5:])
			if data != nil {
				mailboxes = append(mailboxes, data)
			}
		}
	}

	return mailboxes, nil
}

// Status returns the status of a mailbox.
func (c *Client) Status(mailbox string, opts *imap.StatusOptions) (*imap.StatusData, error) {
	items := buildStatusItems(opts)
	c.collectUntagged()

	result, err := c.execute("STATUS", quoteArg(mailbox), "("+strings.Join(items, " ")+")")
	if err != nil {
		return nil, err
	}
	if result.status != "OK" {
		return nil, &imap.IMAPError{StatusResponse: &imap.StatusResponse{
			Type: imap.StatusResponseType(result.status),
			Code: imap.ResponseCode(result.code),
			Text: result.text,
		}}
	}

	// Parse status response from untagged data
	untagged := c.collectUntagged()
	for _, line := range untagged {
		if strings.HasPrefix(line, "STATUS ") {
			return parseStatusResponse2(line[7:]), nil
		}
	}

	return &imap.StatusData{Mailbox: mailbox}, nil
}

// Unselect closes the current mailbox without expunging.
func (c *Client) Unselect() error {
	err := c.executeCheck("UNSELECT")
	if err == nil {
		c.mu.Lock()
		c.state = imap.ConnStateAuthenticated
		c.mailboxName = ""
		c.mu.Unlock()
	}
	return err
}

// CloseMailbox closes the current mailbox and expunges deleted messages.
func (c *Client) CloseMailbox() error {
	err := c.executeCheck("CLOSE")
	if err == nil {
		c.mu.Lock()
		c.state = imap.ConnStateAuthenticated
		c.mailboxName = ""
		c.mu.Unlock()
	}
	return err
}

func buildStatusItems(opts *imap.StatusOptions) []string {
	if opts == nil {
		return []string{"MESSAGES", "UIDNEXT", "UIDVALIDITY", "UNSEEN"}
	}
	var items []string
	if opts.NumMessages {
		items = append(items, "MESSAGES")
	}
	if opts.UIDNext {
		items = append(items, "UIDNEXT")
	}
	if opts.UIDValidity {
		items = append(items, "UIDVALIDITY")
	}
	if opts.NumUnseen {
		items = append(items, "UNSEEN")
	}
	if opts.NumRecent {
		items = append(items, "RECENT")
	}
	if opts.Size {
		items = append(items, "SIZE")
	}
	if opts.HighestModSeq {
		items = append(items, "HIGHESTMODSEQ")
	}
	if len(items) == 0 {
		items = []string{"MESSAGES", "UIDNEXT", "UIDVALIDITY", "UNSEEN"}
	}
	return items
}

func parseListResponse(line string) *imap.ListData {
	// Very simplified LIST response parser
	// Format: (attrs) "delim" mailbox
	data := &imap.ListData{}

	// Parse attributes
	if strings.HasPrefix(line, "(") {
		endParen := strings.IndexByte(line, ')')
		if endParen < 0 {
			return nil
		}
		attrStr := line[1:endParen]
		if attrStr != "" {
			for _, attr := range strings.Fields(attrStr) {
				data.Attrs = append(data.Attrs, imap.MailboxAttr(attr))
			}
		}
		line = strings.TrimLeft(line[endParen+1:], " ")
	}

	// Parse delimiter
	if strings.HasPrefix(line, "NIL") {
		data.Delim = 0
		line = strings.TrimLeft(line[3:], " ")
	} else if strings.HasPrefix(line, `"`) {
		if len(line) >= 3 {
			data.Delim = rune(line[1])
			line = strings.TrimLeft(line[3:], " ")
		}
	}

	// Parse mailbox name
	data.Mailbox = strings.Trim(line, `"`)

	return data
}

func parseStatusResponse2(line string) *imap.StatusData {
	data := &imap.StatusData{}

	// Find mailbox name
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx < 0 {
		return data
	}
	data.Mailbox = strings.Trim(line[:spaceIdx], `"`)
	rest := strings.TrimLeft(line[spaceIdx+1:], " ")

	// Parse status items: (MESSAGES 5 UIDNEXT 10 ...)
	if strings.HasPrefix(rest, "(") {
		rest = strings.TrimPrefix(rest, "(")
		rest = strings.TrimSuffix(rest, ")")
	}

	parts := strings.Fields(rest)
	for i := 0; i+1 < len(parts); i += 2 {
		name := strings.ToUpper(parts[i])
		val, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			continue
		}
		v32 := uint32(val)
		switch name {
		case "MESSAGES":
			data.NumMessages = &v32
		case "UIDNEXT":
			data.UIDNext = &v32
		case "UIDVALIDITY":
			data.UIDValidity = &v32
		case "UNSEEN":
			data.NumUnseen = &v32
		case "RECENT":
			data.NumRecent = &v32
		case "SIZE":
			size := int64(val)
			data.Size = &size
		case "HIGHESTMODSEQ":
			data.HighestModSeq = &val
		}
	}

	return data
}

// Noop sends a NOOP command.
func (c *Client) Noop() error {
	return c.executeCheck("NOOP")
}

// Capability requests the server's capabilities.
func (c *Client) Capability() ([]string, error) {
	c.collectUntagged()
	err := c.executeCheck("CAPABILITY")
	if err != nil {
		return nil, err
	}
	return c.Caps(), nil
}

// Enable enables capabilities.
func (c *Client) Enable(caps ...string) error {
	if len(caps) == 0 {
		return nil
	}
	return c.executeCheck("ENABLE", strings.Join(caps, " "))
}

// Append appends a message to a mailbox.
func (c *Client) Append(mailbox string, flags []imap.Flag, literal []byte) (*imap.AppendData, error) {
	tag := c.tags.Next()
	cmd := c.pending.Add(tag)

	var line strings.Builder
	line.WriteString(tag)
	line.WriteString(" APPEND ")
	line.WriteString(quoteArg(mailbox))

	// Flags
	if len(flags) > 0 {
		line.WriteString(" (")
		for i, f := range flags {
			if i > 0 {
				line.WriteByte(' ')
			}
			line.WriteString(string(f))
		}
		line.WriteByte(')')
	}

	// Literal
	line.WriteString(fmt.Sprintf(" {%d}\r\n", len(literal)))

	c.encoder.RawString(line.String())
	if err := c.encoder.Flush(); err != nil {
		c.pending.Complete(tag, &commandResult{err: err})
		return nil, err
	}

	// Wait for continuation request
	<-c.continuationCh

	// Send the literal data
	_, err := c.conn.Write(literal)
	if err != nil {
		return nil, err
	}
	_, err = c.conn.Write([]byte("\r\n"))
	if err != nil {
		return nil, err
	}

	result := <-cmd.done
	if result.err != nil {
		return nil, result.err
	}
	if result.status != "OK" {
		return nil, &imap.IMAPError{StatusResponse: &imap.StatusResponse{
			Type: imap.StatusResponseType(result.status),
			Code: imap.ResponseCode(result.code),
			Text: result.text,
		}}
	}

	data := &imap.AppendData{}
	// Parse APPENDUID from response code
	if strings.HasPrefix(result.code, "APPENDUID ") {
		parts := strings.Fields(result.code[10:])
		if len(parts) >= 2 {
			if v, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
				data.UIDValidity = uint32(v)
			}
			if v, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
				data.UID = imap.UID(v)
			}
		}
	}

	return data, nil
}
