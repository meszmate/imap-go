// Package pool provides connection pooling for IMAP clients.
package pool

import (
	"errors"
	"sync"

	"github.com/meszmate/imap-go/client"
)

// Pool manages a pool of IMAP client connections.
type Pool struct {
	mu       sync.Mutex
	factory  func() (*client.Client, error)
	clients  []*client.Client
	maxSize  int
	closed   bool
}

// New creates a new connection pool.
func New(maxSize int, factory func() (*client.Client, error)) *Pool {
	return &Pool{
		factory: factory,
		maxSize: maxSize,
	}
}

// Get returns a client from the pool, creating a new one if necessary.
func (p *Pool) Get() (*client.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, errors.New("pool is closed")
	}

	// Try to return an existing client
	if len(p.clients) > 0 {
		c := p.clients[len(p.clients)-1]
		p.clients = p.clients[:len(p.clients)-1]
		return c, nil
	}

	// Create a new one
	return p.factory()
}

// Put returns a client to the pool.
func (p *Pool) Put(c *client.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || len(p.clients) >= p.maxSize {
		c.Close()
		return
	}

	p.clients = append(p.clients, c)
}

// Close closes all clients in the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	for _, c := range p.clients {
		c.Close()
	}
	p.clients = nil
	return nil
}

// Len returns the number of idle clients in the pool.
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients)
}
