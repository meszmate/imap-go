package client

// SupportsIMAP4rev2 returns true if the server supports IMAP4rev2.
func (c *Client) SupportsIMAP4rev2() bool {
	return c.HasCap("IMAP4rev2")
}

// SupportsIdle returns true if the server supports IDLE.
func (c *Client) SupportsIdle() bool {
	return c.HasCap("IDLE")
}

// SupportsMove returns true if the server supports MOVE.
func (c *Client) SupportsMove() bool {
	return c.HasCap("MOVE")
}

// SupportsLiteralPlus returns true if the server supports LITERAL+.
func (c *Client) SupportsLiteralPlus() bool {
	return c.HasCap("LITERAL+")
}

// SupportsUIDPlus returns true if the server supports UIDPLUS.
func (c *Client) SupportsUIDPlus() bool {
	return c.HasCap("UIDPLUS")
}

// SupportsCondStore returns true if the server supports CONDSTORE.
func (c *Client) SupportsCondStore() bool {
	return c.HasCap("CONDSTORE")
}

// SupportsQResync returns true if the server supports QRESYNC.
func (c *Client) SupportsQResync() bool {
	return c.HasCap("QRESYNC")
}

// SupportsNamespace returns true if the server supports NAMESPACE.
func (c *Client) SupportsNamespace() bool {
	return c.HasCap("NAMESPACE")
}

// SupportsSort returns true if the server supports SORT.
func (c *Client) SupportsSort() bool {
	return c.HasCap("SORT")
}

// SupportsID returns true if the server supports ID.
func (c *Client) SupportsID() bool {
	return c.HasCap("ID")
}

// SupportsEnable returns true if the server supports ENABLE.
func (c *Client) SupportsEnable() bool {
	return c.HasCap("ENABLE")
}

// SupportsStartTLS returns true if the server supports STARTTLS.
func (c *Client) SupportsStartTLS() bool {
	return c.HasCap("STARTTLS")
}
