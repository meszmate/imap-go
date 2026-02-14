package client

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIdleRejectedDoesNotHang(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		fmt.Fprint(serverConn, "* OK ready\r\n")

		r := bufio.NewReader(serverConn)
		line, _ := r.ReadString('\n')
		if strings.Contains(line, " IDLE") {
			fmt.Fprint(serverConn, "A1 BAD idle not allowed\r\n")
		}
	}()

	c, err := New(clientConn)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	done := make(chan error, 1)
	go func() {
		_, err := c.Idle()
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Idle() error = nil, want non-nil")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Idle() timed out waiting for tagged rejection")
	}
}

func TestAppendDisconnectWhileWaitingContinuation(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		fmt.Fprint(serverConn, "* OK ready\r\n")

		r := bufio.NewReader(serverConn)
		_, _ = r.ReadString('\n') // APPEND command line with literal size
		_ = serverConn.Close()    // disconnect before continuation
	}()

	c, err := New(clientConn)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	done := make(chan error, 1)
	go func() {
		_, err := c.Append("INBOX", nil, []byte("hello"))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Append() error = nil, want non-nil")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Append() timed out waiting for disconnect")
	}
}

func TestCloseUnblocksIdleWait(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	cmdSeen := make(chan struct{})
	go func() {
		fmt.Fprint(serverConn, "* OK ready\r\n")
		r := bufio.NewReader(serverConn)
		line, _ := r.ReadString('\n')
		if strings.Contains(line, " IDLE") {
			close(cmdSeen)
		}
		_, _ = r.ReadString('\n')
	}()

	c, err := New(clientConn)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	done := make(chan error, 1)
	go func() {
		_, err := c.Idle()
		done <- err
	}()

	select {
	case <-cmdSeen:
	case <-time.After(1 * time.Second):
		t.Fatal("server did not receive IDLE command")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Idle() error = nil after Close(), want non-nil")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Idle() timed out after Close()")
	}
}

func TestDoneClosedOnServerDisconnect(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		fmt.Fprint(serverConn, "* OK ready\r\n")
		_ = serverConn.Close()
	}()

	c, err := New(clientConn)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	select {
	case <-c.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("Done() was not closed after server disconnect")
	}

	if err := c.DisconnectErr(); err == nil {
		t.Fatal("DisconnectErr() = nil, want non-nil")
	}
}

func TestDoneClosedOnClientClose(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		fmt.Fprint(serverConn, "* OK ready\r\n")
		r := bufio.NewReader(serverConn)
		_, _ = r.ReadString('\n')
	}()

	c, err := New(clientConn)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	select {
	case <-c.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("Done() was not closed after Close()")
	}

	if err := c.DisconnectErr(); err == nil {
		t.Fatal("DisconnectErr() = nil, want non-nil")
	}
}
