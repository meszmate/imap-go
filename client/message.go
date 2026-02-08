package client

import (
	"fmt"
	"strconv"
	"strings"

	imap "github.com/meszmate/imap-go"
)

// Fetch retrieves message data for the given sequence set.
func (c *Client) Fetch(seqSet string, items string) ([]string, error) {
	c.collectUntagged()

	result, err := c.execute("FETCH", seqSet, items)
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

	var responses []string
	untagged := c.collectUntagged()
	for _, line := range untagged {
		if strings.HasPrefix(line, "FETCH ") {
			responses = append(responses, line)
		}
	}
	return responses, nil
}

// UIDFetch retrieves message data using UIDs.
func (c *Client) UIDFetch(uidSet string, items string) ([]string, error) {
	c.collectUntagged()

	result, err := c.execute("UID FETCH", uidSet, items)
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

	var responses []string
	untagged := c.collectUntagged()
	for _, line := range untagged {
		if strings.HasPrefix(line, "FETCH ") {
			responses = append(responses, line)
		}
	}
	return responses, nil
}

// Store modifies message flags.
func (c *Client) Store(seqSet string, action imap.StoreAction, flags []imap.Flag, silent bool) error {
	item := action.String()
	if silent {
		item += ".SILENT"
	}

	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	flagList := "(" + strings.Join(flagStrs, " ") + ")"

	return c.executeCheck("STORE", seqSet, item, flagList)
}

// UIDStore modifies message flags using UIDs.
func (c *Client) UIDStore(uidSet string, action imap.StoreAction, flags []imap.Flag, silent bool) error {
	item := action.String()
	if silent {
		item += ".SILENT"
	}

	flagStrs := make([]string, len(flags))
	for i, f := range flags {
		flagStrs[i] = string(f)
	}
	flagList := "(" + strings.Join(flagStrs, " ") + ")"

	return c.executeCheck("UID STORE", uidSet, item, flagList)
}

// Copy copies messages to another mailbox.
func (c *Client) Copy(seqSet, dest string) (*imap.CopyData, error) {
	result, err := c.execute("COPY", seqSet, quoteArg(dest))
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

	data := &imap.CopyData{}
	if strings.HasPrefix(result.code, "COPYUID ") {
		parseCopyUID(result.code[8:], data)
	}
	return data, nil
}

// UIDCopy copies messages using UIDs.
func (c *Client) UIDCopy(uidSet, dest string) (*imap.CopyData, error) {
	result, err := c.execute("UID COPY", uidSet, quoteArg(dest))
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

	data := &imap.CopyData{}
	if strings.HasPrefix(result.code, "COPYUID ") {
		parseCopyUID(result.code[8:], data)
	}
	return data, nil
}

// Move moves messages to another mailbox (MOVE extension).
func (c *Client) Move(seqSet, dest string) (*imap.CopyData, error) {
	result, err := c.execute("MOVE", seqSet, quoteArg(dest))
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

	data := &imap.CopyData{}
	if strings.HasPrefix(result.code, "COPYUID ") {
		parseCopyUID(result.code[8:], data)
	}
	return data, nil
}

// Expunge permanently removes deleted messages.
func (c *Client) Expunge() error {
	return c.executeCheck("EXPUNGE")
}

// UIDExpunge permanently removes specified UIDs (UIDPLUS).
func (c *Client) UIDExpunge(uidSet string) error {
	return c.executeCheck("UID EXPUNGE", uidSet)
}

// Search searches for messages matching criteria.
func (c *Client) Search(criteria string) ([]uint32, error) {
	c.collectUntagged()

	result, err := c.execute("SEARCH", criteria)
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

	return parseSearchResults(c.collectUntagged()), nil
}

// UIDSearch searches using UIDs.
func (c *Client) UIDSearch(criteria string) ([]uint32, error) {
	c.collectUntagged()

	result, err := c.execute("UID SEARCH", criteria)
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

	return parseSearchResults(c.collectUntagged()), nil
}

func parseCopyUID(s string, data *imap.CopyData) {
	parts := strings.Fields(s)
	if len(parts) >= 3 {
		if v, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
			data.UIDValidity = uint32(v)
		}
		if set, err := imap.ParseUIDSet(parts[1]); err == nil {
			data.SourceUIDs = *set
		}
		if set, err := imap.ParseUIDSet(parts[2]); err == nil {
			data.DestUIDs = *set
		}
	}
}

func parseSearchResults(lines []string) []uint32 {
	var results []uint32
	for _, line := range lines {
		if strings.HasPrefix(line, "SEARCH ") {
			fields := strings.Fields(line[7:])
			for _, f := range fields {
				if n, err := strconv.ParseUint(f, 10, 32); err == nil {
					results = append(results, uint32(n))
				}
			}
		}
	}
	return results
}

// Sort sorts messages (SORT extension).
func (c *Client) Sort(criteria string) ([]uint32, error) {
	c.collectUntagged()

	result, err := c.execute("SORT", criteria)
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

	var results []uint32
	for _, line := range c.collectUntagged() {
		if strings.HasPrefix(line, "SORT ") {
			fields := strings.Fields(line[5:])
			for _, f := range fields {
				if n, err := strconv.ParseUint(f, 10, 32); err == nil {
					results = append(results, uint32(n))
				}
			}
		}
	}
	return results, nil
}

// Thread retrieves threading information (THREAD extension).
func (c *Client) Thread(algorithm, criteria string) ([]string, error) {
	c.collectUntagged()

	result, err := c.execute("THREAD", algorithm, criteria)
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

	var results []string
	for _, line := range c.collectUntagged() {
		if strings.HasPrefix(line, "THREAD ") {
			results = append(results, line[7:])
		}
	}
	return results, nil
}

// ID sends an ID command (RFC 2971).
func (c *Client) ID(clientID map[string]string) (map[string]string, error) {
	c.collectUntagged()

	var args string
	if clientID == nil {
		args = "NIL"
	} else {
		var parts []string
		for k, v := range clientID {
			parts = append(parts, fmt.Sprintf("%q %q", k, v))
		}
		args = "(" + strings.Join(parts, " ") + ")"
	}

	result, err := c.execute("ID", args)
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

	// Parse ID response from untagged data
	_ = c.collectUntagged()
	return nil, nil // Simplified - full parsing would be complex
}
