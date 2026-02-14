// Package client implements an IMAP client.
//
// The client supports pipelining (sending multiple commands before waiting
// for responses), automatic capability negotiation, and extensible
// response handling.
package client

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/wire"
)

// Client is an IMAP client.
type Client struct {
	conn    net.Conn
	encoder *wire.Encoder
	decoder *wire.Decoder
	options *Options
	tags    *tagGenerator
	pending *pendingCommands
	reader  *reader

	mu                 sync.Mutex
	state              imap.ConnState
	caps               []string
	mailboxName        string
	mailboxMessages    uint32
	mailboxRecent      uint32
	mailboxUIDValidity uint32
	mailboxUIDNext     uint32
	mailboxUnseen      uint32
	mailboxReadOnly    bool

	// untaggedData collects untagged responses for the current command
	untaggedMu   sync.Mutex
	untaggedData []string

	// continuationCh is used to signal continuation requests to waiting commands
	continuationCh chan continuation

	closed         bool
	disconnectOnce sync.Once
	disconnectCh   chan struct{}
	disconnectErr  error
}

type continuation struct {
	text string
	err  error
}

// New creates a new Client from an existing connection.
// The caller is responsible for reading the server greeting before calling this.
func New(conn net.Conn, opts ...Option) (*Client, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	c := &Client{
		conn:           conn,
		encoder:        wire.NewEncoder(conn),
		decoder:        wire.NewDecoder(conn),
		options:        options,
		tags:           newTagGenerator("A"),
		pending:        newPendingCommands(),
		continuationCh: make(chan continuation, 1),
		disconnectCh:   make(chan struct{}),
		state:          imap.ConnStateNotAuthenticated,
	}

	// Read the server greeting
	line, err := c.decoder.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading greeting: %w", err)
	}

	c.options.Logger.Debug("greeting", "line", line)

	// Parse greeting
	if strings.HasPrefix(line, "* OK") {
		c.state = imap.ConnStateNotAuthenticated
	} else if strings.HasPrefix(line, "* PREAUTH") {
		c.state = imap.ConnStateAuthenticated
	} else if strings.HasPrefix(line, "* BYE") {
		return nil, fmt.Errorf("server rejected connection: %s", line)
	} else {
		return nil, fmt.Errorf("unexpected greeting: %s", line)
	}

	// Parse capabilities from greeting if present
	if bracketIdx := strings.Index(line, "[CAPABILITY "); bracketIdx >= 0 {
		end := strings.IndexByte(line[bracketIdx:], ']')
		if end > 0 {
			capStr := line[bracketIdx+12 : bracketIdx+end]
			c.caps = strings.Fields(capStr)
		}
	}

	// Start the background reader
	c.reader = newReader(c.decoder, c)
	go c.reader.run()

	return c, nil
}

// Dial connects to an IMAP server at the given address.
func Dial(addr string, opts ...Option) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return New(conn, opts...)
}

// DialTLS connects to an IMAP server using TLS.
func DialTLS(addr string, config *tls.Config, opts ...Option) (*Client, error) {
	conn, err := tls.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("dial TLS: %w", err)
	}
	return New(conn, opts...)
}

// State returns the current connection state.
func (c *Client) State() imap.ConnState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Caps returns the server's capabilities.
func (c *Client) Caps() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]string, len(c.caps))
	copy(result, c.caps)
	return result
}

// HasCap returns true if the server advertises the given capability.
func (c *Client) HasCap(cap string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	upper := strings.ToUpper(cap)
	for _, s := range c.caps {
		if strings.ToUpper(s) == upper {
			return true
		}
	}
	return false
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	err := c.conn.Close()
	c.handleDisconnect(errors.New("connection closed"))
	return err
}

// execute sends a command and waits for the tagged response.
func (c *Client) execute(name string, args ...string) (*commandResult, error) {
	tag := c.tags.Next()
	cmd := c.pending.Add(tag)

	// Build the command line
	var line strings.Builder
	line.WriteString(tag)
	line.WriteByte(' ')
	line.WriteString(name)
	for _, arg := range args {
		line.WriteByte(' ')
		line.WriteString(arg)
	}
	line.WriteString("\r\n")

	c.options.Logger.Debug("send", "line", strings.TrimRight(line.String(), "\r\n"))

	// Write the command
	c.encoder.RawString(line.String())
	if err := c.encoder.Flush(); err != nil {
		c.pending.Complete(tag, &commandResult{err: err})
		return nil, err
	}

	// Wait for the result
	result := <-cmd.done
	if result.err != nil {
		return nil, result.err
	}

	return result, nil
}

// executeCheck executes a command and returns an error if the response is not OK.
func (c *Client) executeCheck(name string, args ...string) error {
	result, err := c.execute(name, args...)
	if err != nil {
		return err
	}
	return commandResultError(result)
}

// collectUntagged returns and clears collected untagged data.
func (c *Client) collectUntagged() []string {
	c.untaggedMu.Lock()
	defer c.untaggedMu.Unlock()
	data := c.untaggedData
	c.untaggedData = nil
	return data
}

// storeUntagged adds an untagged response to the collection.
func (c *Client) storeUntagged(line string) {
	c.untaggedMu.Lock()
	c.untaggedData = append(c.untaggedData, line)
	c.untaggedMu.Unlock()
}

// handleContinuation processes a continuation request.
func (c *Client) handleContinuation(line string) {
	text := ""
	if len(line) > 2 {
		text = line[2:]
	}
	select {
	case c.continuationCh <- continuation{text: text}:
	default:
	}
}

func (c *Client) handleDisconnect(err error) {
	if err == nil {
		err = errors.New("connection closed")
	}

	c.disconnectOnce.Do(func() {
		c.mu.Lock()
		c.disconnectErr = err
		c.mu.Unlock()

		c.pending.CompleteAll(fmt.Errorf("connection closed: %w", err))
		select {
		case c.continuationCh <- continuation{err: fmt.Errorf("connection closed: %w", err)}:
		default:
		}
		close(c.disconnectCh)
	})
}

// Done returns a channel that is closed when the client disconnects.
func (c *Client) Done() <-chan struct{} {
	return c.disconnectCh
}

// DisconnectErr returns the disconnect cause after Done is closed.
func (c *Client) DisconnectErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnectErr
}

func commandResultError(result *commandResult) error {
	if result == nil {
		return errors.New("missing command result")
	}
	if result.err != nil {
		return result.err
	}
	if result.status == "OK" {
		return nil
	}
	return &imap.IMAPError{StatusResponse: &imap.StatusResponse{
		Type: imap.StatusResponseType(result.status),
		Code: imap.ResponseCode(result.code),
		Text: result.text,
	}}
}

func (c *Client) waitForContinuation(cmd *pendingCommand) (string, error) {
	for {
		select {
		case cont := <-c.continuationCh:
			if cont.err != nil {
				return "", cont.err
			}
			return cont.text, nil
		case result := <-cmd.done:
			if err := commandResultError(result); err != nil {
				return "", err
			}
			return "", errors.New("missing continuation request")
		}
	}
}

// Writer returns the underlying encoder for advanced use.
func (c *Client) Writer() io.Writer {
	return c.conn
}
