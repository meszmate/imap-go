package client

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// pendingCommand represents a command awaiting its tagged response.
type pendingCommand struct {
	tag    string
	done   chan *commandResult
}

// commandResult is the result of a completed command.
type commandResult struct {
	status string // "OK", "NO", "BAD"
	code   string // response code (may be empty)
	text   string // human-readable text
	err    error  // non-nil if an error occurred before getting a response
}

// tagGenerator generates unique command tags.
type tagGenerator struct {
	counter atomic.Int64
	prefix  string
}

// newTagGenerator creates a new tag generator.
func newTagGenerator(prefix string) *tagGenerator {
	return &tagGenerator{prefix: prefix}
}

// Next returns the next unique tag.
func (g *tagGenerator) Next() string {
	n := g.counter.Add(1)
	return fmt.Sprintf("%s%d", g.prefix, n)
}

// pendingCommands tracks commands awaiting responses.
type pendingCommands struct {
	mu       sync.Mutex
	commands map[string]*pendingCommand
}

func newPendingCommands() *pendingCommands {
	return &pendingCommands{
		commands: make(map[string]*pendingCommand),
	}
}

// Add registers a new pending command and returns it.
func (pc *pendingCommands) Add(tag string) *pendingCommand {
	cmd := &pendingCommand{
		tag:  tag,
		done: make(chan *commandResult, 1),
	}
	pc.mu.Lock()
	pc.commands[tag] = cmd
	pc.mu.Unlock()
	return cmd
}

// Complete completes a pending command with the given result.
func (pc *pendingCommands) Complete(tag string, result *commandResult) {
	pc.mu.Lock()
	cmd, ok := pc.commands[tag]
	if ok {
		delete(pc.commands, tag)
	}
	pc.mu.Unlock()

	if ok {
		cmd.done <- result
	}
}

// CompleteAll completes all pending commands with an error.
func (pc *pendingCommands) CompleteAll(err error) {
	pc.mu.Lock()
	commands := pc.commands
	pc.commands = make(map[string]*pendingCommand)
	pc.mu.Unlock()

	for _, cmd := range commands {
		cmd.done <- &commandResult{err: err}
	}
}
