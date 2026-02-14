package client

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/meszmate/imap-go/wire"
)

// reader is the background goroutine that reads responses from the server.
type reader struct {
	decoder *wire.Decoder
	client  *Client
}

func newReader(decoder *wire.Decoder, c *Client) *reader {
	return &reader{
		decoder: decoder,
		client:  c,
	}
}

// run reads and dispatches server responses until the connection is closed.
func (r *reader) run() {
	for {
		line, err := r.decoder.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			r.client.options.Logger.Debug("reader error", "error", err)
			r.client.handleDisconnect(err)
			return
		}

		r.client.options.Logger.Debug("recv", "line", line)

		if err := r.processLine(line); err != nil {
			r.client.options.Logger.Debug("process error", "error", err)
		}
	}
}

// processLine handles a single response line.
func (r *reader) processLine(line string) error {
	if len(line) == 0 {
		return nil
	}

	// Continuation request
	if line[0] == '+' {
		r.client.handleContinuation(line)
		return nil
	}

	// Untagged response
	if strings.HasPrefix(line, "* ") {
		return r.processUntagged(line[2:])
	}

	// Tagged response
	return r.processTagged(line)
}

// processUntagged handles an untagged response.
func (r *reader) processUntagged(line string) error {
	// Try to parse as numeric response: "123 EXISTS", "456 EXPUNGE", etc.
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx > 0 {
		numStr := line[:spaceIdx]
		if num, err := strconv.ParseUint(numStr, 10, 32); err == nil {
			rest := line[spaceIdx+1:]
			return r.processNumericResponse(uint32(num), rest)
		}
	}

	// Named response
	upperLine := strings.ToUpper(line)

	if strings.HasPrefix(upperLine, "OK ") {
		r.handleStatusResponse("OK", line[3:])
		return nil
	}
	if strings.HasPrefix(upperLine, "NO ") {
		r.handleStatusResponse("NO", line[3:])
		return nil
	}
	if strings.HasPrefix(upperLine, "BAD ") {
		r.handleStatusResponse("BAD", line[4:])
		return nil
	}
	if strings.HasPrefix(upperLine, "BYE ") {
		r.handleStatusResponse("BYE", line[4:])
		return nil
	}
	if strings.HasPrefix(upperLine, "PREAUTH ") {
		r.handleStatusResponse("PREAUTH", line[8:])
		return nil
	}
	if strings.HasPrefix(upperLine, "CAPABILITY ") {
		r.handleCapability(line[11:])
		return nil
	}
	if strings.HasPrefix(upperLine, "FLAGS ") {
		r.handleFlags(line[6:])
		return nil
	}
	if strings.HasPrefix(upperLine, "LIST ") {
		r.handleList(line[5:])
		return nil
	}
	if strings.HasPrefix(upperLine, "LSUB ") {
		r.handleList(line[5:])
		return nil
	}
	if strings.HasPrefix(upperLine, "STATUS ") {
		r.handleStatus(line[7:])
		return nil
	}
	if strings.HasPrefix(upperLine, "SEARCH ") || upperLine == "SEARCH" {
		r.handleSearch(line)
		return nil
	}
	if strings.HasPrefix(upperLine, "ESEARCH ") {
		r.handleESearch(line[8:])
		return nil
	}
	if strings.HasPrefix(upperLine, "NAMESPACE ") {
		r.handleNamespace(line[10:])
		return nil
	}

	// Store for any waiting data collector
	r.client.storeUntagged(line)
	return nil
}

// processNumericResponse handles "* 123 SOMETHING" responses.
func (r *reader) processNumericResponse(num uint32, rest string) error {
	upper := strings.ToUpper(rest)

	switch {
	case upper == "EXISTS":
		r.client.mu.Lock()
		r.client.mailboxMessages = num
		r.client.mu.Unlock()
		if h := r.client.options.UnilateralDataHandler; h != nil && h.Exists != nil {
			h.Exists(num)
		}
	case upper == "RECENT":
		r.client.mu.Lock()
		r.client.mailboxRecent = num
		r.client.mu.Unlock()
		if h := r.client.options.UnilateralDataHandler; h != nil && h.Recent != nil {
			h.Recent(num)
		}
	case upper == "EXPUNGE":
		if h := r.client.options.UnilateralDataHandler; h != nil && h.Expunge != nil {
			h.Expunge(num)
		}
	case strings.HasPrefix(upper, "FETCH "):
		r.handleFetchResponse(num, rest[6:])
	default:
		r.client.storeUntagged(fmt.Sprintf("%d %s", num, rest))
	}

	return nil
}

// processTagged handles a tagged response (completes a pending command).
func (r *reader) processTagged(line string) error {
	// Format: TAG STATUS [CODE] text
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx < 0 {
		return fmt.Errorf("malformed tagged response: %q", line)
	}

	tag := line[:spaceIdx]
	rest := line[spaceIdx+1:]

	status, code, text := parseStatusResponse(rest)

	r.client.pending.Complete(tag, &commandResult{
		status: status,
		code:   code,
		text:   text,
	})

	return nil
}

func parseStatusResponse(s string) (status, code, text string) {
	spaceIdx := strings.IndexByte(s, ' ')
	if spaceIdx < 0 {
		return s, "", ""
	}
	status = s[:spaceIdx]
	rest := s[spaceIdx+1:]

	if strings.HasPrefix(rest, "[") {
		endBracket := strings.IndexByte(rest, ']')
		if endBracket > 0 {
			code = rest[1:endBracket]
			if endBracket+2 < len(rest) {
				text = rest[endBracket+2:]
			}
			return
		}
	}

	text = rest
	return
}

// Stub handlers - these store data for the client to consume

func (r *reader) handleStatusResponse(status, text string) {
	// Parse response code if present
	if strings.HasPrefix(text, "[") {
		endBracket := strings.IndexByte(text, ']')
		if endBracket > 0 {
			code := text[1:endBracket]
			r.handleResponseCode(code)
		}
	}
}

func (r *reader) handleResponseCode(code string) {
	upper := strings.ToUpper(code)

	parts := strings.SplitN(code, " ", 2)
	name := strings.ToUpper(parts[0])
	var arg string
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch name {
	case "UIDVALIDITY":
		if n, err := strconv.ParseUint(arg, 10, 32); err == nil {
			r.client.mu.Lock()
			r.client.mailboxUIDValidity = uint32(n)
			r.client.mu.Unlock()
		}
	case "UIDNEXT":
		if n, err := strconv.ParseUint(arg, 10, 32); err == nil {
			r.client.mu.Lock()
			r.client.mailboxUIDNext = uint32(n)
			r.client.mu.Unlock()
		}
	case "UNSEEN":
		if n, err := strconv.ParseUint(arg, 10, 32); err == nil {
			r.client.mu.Lock()
			r.client.mailboxUnseen = uint32(n)
			r.client.mu.Unlock()
		}
	case "PERMANENTFLAGS":
		r.client.storeUntagged("PERMANENTFLAGS " + arg)
	case "CAPABILITY":
		r.handleCapability(arg)
	case "READ-ONLY":
		r.client.mu.Lock()
		r.client.mailboxReadOnly = true
		r.client.mu.Unlock()
	case "READ-WRITE":
		r.client.mu.Lock()
		r.client.mailboxReadOnly = false
		r.client.mu.Unlock()
	default:
		_ = upper
	}
}

func (r *reader) handleCapability(line string) {
	caps := strings.Fields(line)
	r.client.mu.Lock()
	r.client.caps = caps
	r.client.mu.Unlock()
}

func (r *reader) handleFlags(line string) {
	r.client.storeUntagged("FLAGS " + line)
}

func (r *reader) handleList(line string) {
	r.client.storeUntagged("LIST " + line)
}

func (r *reader) handleStatus(line string) {
	r.client.storeUntagged("STATUS " + line)
}

func (r *reader) handleSearch(line string) {
	r.client.storeUntagged("SEARCH " + line)
}

func (r *reader) handleESearch(line string) {
	r.client.storeUntagged("ESEARCH " + line)
}

func (r *reader) handleNamespace(line string) {
	r.client.storeUntagged("NAMESPACE " + line)
}

func (r *reader) handleFetchResponse(seqNum uint32, data string) {
	r.client.storeUntagged(fmt.Sprintf("FETCH %d %s", seqNum, data))
}
