package client

import "strings"

// IdleCommand represents an in-progress IDLE command.
type IdleCommand struct {
	tag    string
	client *Client
	cmd    *pendingCommand
}

// Idle starts an IDLE command. Call Done() on the returned IdleCommand to stop.
func (c *Client) Idle() (*IdleCommand, error) {
	tag := c.tags.Next()
	cmd := c.pending.Add(tag)

	var line strings.Builder
	line.WriteString(tag)
	line.WriteString(" IDLE\r\n")

	c.encoder.RawString(line.String())
	if err := c.encoder.Flush(); err != nil {
		c.pending.Complete(tag, &commandResult{err: err})
		return nil, err
	}

	// Wait for continuation request
	if _, err := c.waitForContinuation(cmd); err != nil {
		return nil, err
	}

	return &IdleCommand{
		tag:    tag,
		client: c,
		cmd:    cmd,
	}, nil
}

// Wait blocks until the IDLE command completes or is stopped.
func (ic *IdleCommand) Wait() error {
	result := <-ic.cmd.done
	if err := commandResultError(result); err != nil {
		return err
	}
	return nil
}

// Done sends the DONE command to stop IDLE.
func (ic *IdleCommand) Done() error {
	ic.client.encoder.RawString("DONE\r\n")
	if err := ic.client.encoder.Flush(); err != nil {
		return err
	}
	return ic.Wait()
}
