package client

import (
	"crypto/tls"
	"fmt"

	"github.com/meszmate/imap-go/wire"
)

// StartTLS upgrades the connection to TLS.
func (c *Client) StartTLS(config *tls.Config) error {
	if config == nil {
		config = c.options.TLSConfig
	}
	if config == nil {
		return fmt.Errorf("TLS config required")
	}

	err := c.executeCheck("STARTTLS")
	if err != nil {
		return err
	}

	// Upgrade the connection
	tlsConn := tls.Client(c.conn, config)
	if err := tlsConn.Handshake(); err != nil {
		return fmt.Errorf("TLS handshake: %w", err)
	}

	c.mu.Lock()
	c.conn = tlsConn
	c.encoder = wire.NewEncoder(tlsConn)
	c.decoder = wire.NewDecoder(tlsConn)
	c.mu.Unlock()

	// Re-start the reader with the new decoder
	c.reader = newReader(c.decoder, c)
	go c.reader.run()

	return nil
}
