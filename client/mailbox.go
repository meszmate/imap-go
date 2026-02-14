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

// CreateWithOptions creates a new mailbox with options.
// If options includes a SpecialUse attribute, the USE parameter is sent
// per RFC 6154: CREATE mailbox (USE (\Sent))
func (c *Client) CreateWithOptions(mailbox string, options *imap.CreateOptions) error {
	args := []string{quoteArg(mailbox)}
	if options != nil && options.SpecialUse != "" {
		args = append(args, "(USE ("+string(options.SpecialUse)+"))")
	}
	return c.executeCheck("CREATE", args...)
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

// ListMailboxesExtended lists mailboxes with extended LIST options (RFC 5258).
func (c *Client) ListMailboxesExtended(ref string, patterns []string, options *imap.ListOptions) ([]*imap.ListData, error) {
	c.collectUntagged()

	// Build command arguments
	var args []string

	// Selection options
	if options != nil && hasSelectionOpts(options) {
		var selOpts []string
		if options.SelectSubscribed {
			selOpts = append(selOpts, "SUBSCRIBED")
		}
		if options.SelectRemote {
			selOpts = append(selOpts, "REMOTE")
		}
		if options.SelectRecursiveMatch {
			selOpts = append(selOpts, "RECURSIVEMATCH")
		}
		if options.SelectSpecialUse {
			selOpts = append(selOpts, "SPECIAL-USE")
		}
		args = append(args, "("+strings.Join(selOpts, " ")+")")
	}

	// Reference name
	args = append(args, quoteArg(ref))

	// Patterns
	if len(patterns) == 1 {
		args = append(args, quoteArg(patterns[0]))
	} else {
		var patternParts []string
		for _, p := range patterns {
			patternParts = append(patternParts, quoteArg(p))
		}
		args = append(args, "("+strings.Join(patternParts, " ")+")")
	}

	// Return options
	if options != nil && hasReturnOpts(options) {
		var retOpts []string
		if options.ReturnSubscribed {
			retOpts = append(retOpts, "SUBSCRIBED")
		}
		if options.ReturnChildren {
			retOpts = append(retOpts, "CHILDREN")
		}
		if options.ReturnSpecialUse {
			retOpts = append(retOpts, "SPECIAL-USE")
		}
		if options.ReturnMyRights {
			retOpts = append(retOpts, "MYRIGHTS")
		}
		if options.ReturnStatus != nil {
			items := buildStatusItems(options.ReturnStatus)
			retOpts = append(retOpts, "STATUS ("+strings.Join(items, " ")+")")
		}
		if options.ReturnMetadata != nil {
			var metaParts []string
			for _, opt := range options.ReturnMetadata.Options {
				metaParts = append(metaParts, quoteArg(opt))
			}
			if options.ReturnMetadata.MaxSize > 0 {
				metaParts = append(metaParts, fmt.Sprintf("MAXSIZE %d", options.ReturnMetadata.MaxSize))
			}
			if options.ReturnMetadata.Depth != "" {
				metaParts = append(metaParts, "DEPTH "+options.ReturnMetadata.Depth)
			}
			retOpts = append(retOpts, "METADATA ("+strings.Join(metaParts, " ")+")")
		}
		args = append(args, "RETURN", "("+strings.Join(retOpts, " ")+")")
	}

	result, err := c.execute("LIST", args...)
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

	// Build a map for matching STATUS responses to LIST entries
	mailboxMap := make(map[string]*imap.ListData)

	for _, line := range untagged {
		if strings.HasPrefix(line, "LIST ") {
			data := parseListResponse(line[5:])
			if data != nil {
				mailboxes = append(mailboxes, data)
				mailboxMap[data.Mailbox] = data
			}
		}
	}

	// Match STATUS responses to mailboxes
	for _, line := range untagged {
		if strings.HasPrefix(line, "STATUS ") {
			statusData := parseStatusResponse2(line[7:])
			if statusData != nil {
				if ld, ok := mailboxMap[statusData.Mailbox]; ok {
					ld.Status = statusData
				}
			}
		}
	}

	return mailboxes, nil
}

func hasSelectionOpts(opts *imap.ListOptions) bool {
	return opts.SelectSubscribed || opts.SelectRemote || opts.SelectRecursiveMatch || opts.SelectSpecialUse
}

func hasReturnOpts(opts *imap.ListOptions) bool {
	return opts.ReturnSubscribed || opts.ReturnChildren || opts.ReturnSpecialUse ||
		opts.ReturnMyRights || opts.ReturnStatus != nil || opts.ReturnMetadata != nil
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
	// Format: (attrs) "delim" mailbox [extended-data]
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

	// Parse mailbox name — may be quoted or unquoted
	mailbox, rest := parseMailboxName(line)
	data.Mailbox = mailbox

	// Parse extended data if present
	rest = strings.TrimLeft(rest, " ")
	if strings.HasPrefix(rest, "(") {
		parseExtendedData(rest, data)
	}

	return data
}

// parseMailboxName extracts the mailbox name from the remaining line.
// Returns the mailbox name and the rest of the line after it.
func parseMailboxName(line string) (string, string) {
	if strings.HasPrefix(line, `"`) {
		// Quoted mailbox name
		end := 1
		for end < len(line) {
			if line[end] == '\\' && end+1 < len(line) {
				end += 2
				continue
			}
			if line[end] == '"' {
				return line[1:end], line[end+1:]
			}
			end++
		}
		return strings.Trim(line, `"`), ""
	}

	// Unquoted (atom) mailbox name — ends at SP or end of line
	idx := strings.IndexByte(line, ' ')
	if idx < 0 {
		return line, ""
	}
	return line[:idx], line[idx:]
}

// parseExtendedData parses extended data items from a parenthesized list.
// Format: ("CHILDINFO" ("SUBSCRIBED") "OLDNAME" ("OldName") ...)
func parseExtendedData(s string, data *imap.ListData) {
	// Remove outer parentheses
	if len(s) < 2 || s[0] != '(' {
		return
	}
	// Find matching close paren
	depth := 0
	end := -1
	for i, ch := range s {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return
	}
	inner := s[1:end]

	// Parse key-value extended items
	for len(inner) > 0 {
		inner = strings.TrimLeft(inner, " ")
		if inner == "" {
			break
		}

		// Read key (quoted string like "CHILDINFO")
		key, rest := readQuotedOrAtom(inner)
		inner = strings.TrimLeft(rest, " ")
		key = strings.ToUpper(key)

		switch key {
		case "CHILDINFO":
			// Value is a parenthesized list of quoted strings
			if strings.HasPrefix(inner, "(") {
				listStr, rest2 := extractParenthesized(inner)
				inner = strings.TrimLeft(rest2, " ")
				// Parse items inside the parens
				listInner := listStr
				for len(listInner) > 0 {
					listInner = strings.TrimLeft(listInner, " ")
					if listInner == "" {
						break
					}
					val, r := readQuotedOrAtom(listInner)
					data.ChildInfo = append(data.ChildInfo, val)
					listInner = r
				}
			}
		case "OLDNAME":
			// Value is a parenthesized mailbox name
			if strings.HasPrefix(inner, "(") {
				listStr, rest2 := extractParenthesized(inner)
				inner = strings.TrimLeft(rest2, " ")
				name, _ := readQuotedOrAtom(strings.TrimSpace(listStr))
				data.OldName = name
			}
		case "MYRIGHTS":
			// Value is a quoted string
			val, rest2 := readQuotedOrAtom(inner)
			data.MyRights = val
			inner = strings.TrimLeft(rest2, " ")
		case "METADATA":
			// Value is a parenthesized list of key-value pairs
			if strings.HasPrefix(inner, "(") {
				listStr, rest2 := extractParenthesized(inner)
				inner = strings.TrimLeft(rest2, " ")
				data.Metadata = make(map[string]string)
				listInner := listStr
				for len(listInner) > 0 {
					listInner = strings.TrimLeft(listInner, " ")
					if listInner == "" {
						break
					}
					key, r := readQuotedOrAtom(listInner)
					r = strings.TrimLeft(r, " ")
					val, r2 := readQuotedOrAtom(r)
					data.Metadata[key] = val
					listInner = strings.TrimLeft(r2, " ")
				}
			}
		}
	}
}

// readQuotedOrAtom reads a quoted string or atom from the beginning of s.
// Returns the value and the remaining string.
func readQuotedOrAtom(s string) (string, string) {
	if len(s) == 0 {
		return "", ""
	}
	if s[0] == '"' {
		// Quoted string
		end := 1
		for end < len(s) {
			if s[end] == '\\' && end+1 < len(s) {
				end += 2
				continue
			}
			if s[end] == '"' {
				return s[1:end], s[end+1:]
			}
			end++
		}
		return s[1:], ""
	}

	// Atom: read until space, paren, or end
	end := 0
	for end < len(s) && s[end] != ' ' && s[end] != '(' && s[end] != ')' {
		end++
	}
	return s[:end], s[end:]
}

// extractParenthesized extracts content between matching parentheses.
// Input should start with '('. Returns the inner content and remaining string.
func extractParenthesized(s string) (string, string) {
	if len(s) == 0 || s[0] != '(' {
		return "", s
	}
	depth := 0
	for i := range s {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:]
			}
		}
	}
	return s[1:], ""
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
	if _, err := c.waitForContinuation(cmd); err != nil {
		return nil, err
	}

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
	if err := commandResultError(result); err != nil {
		return nil, err
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
