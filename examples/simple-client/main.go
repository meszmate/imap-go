// Command simple-client demonstrates basic IMAP client usage.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/meszmate/imap-go/client"
)

func main() {
	addr := "imap.example.com:993"
	user := "user@example.com"
	pass := "password"

	if len(os.Args) >= 4 {
		addr = os.Args[1]
		user = os.Args[2]
		pass = os.Args[3]
	}

	// Connect with TLS
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = c.Close() }()

	fmt.Printf("Connected to %s\n", addr)
	fmt.Printf("Capabilities: %v\n", c.Caps())

	// Login
	if err := c.Login(user, pass); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Println("Logged in successfully")

	// List mailboxes
	mailboxes, err := c.ListMailboxes("", "*")
	if err != nil {
		log.Fatalf("List failed: %v", err)
	}
	fmt.Printf("\nMailboxes (%d):\n", len(mailboxes))
	for _, mbox := range mailboxes {
		fmt.Printf("  %s (delim=%c, attrs=%v)\n", mbox.Mailbox, mbox.Delim, mbox.Attrs)
	}

	// Select INBOX
	selectData, err := c.Select("INBOX", nil)
	if err != nil {
		log.Fatalf("Select failed: %v", err)
	}
	fmt.Printf("\nINBOX: %d messages, UID next: %d\n", selectData.NumMessages, selectData.UIDNext)

	// Fetch first 10 messages (envelope)
	if selectData.NumMessages > 0 {
		end := selectData.NumMessages
		if end > 10 {
			end = 10
		}

		fetchResult, err := c.Fetch(fmt.Sprintf("1:%d", end), "(FLAGS UID ENVELOPE)")
		if err != nil {
			log.Fatalf("Fetch failed: %v", err)
		}

		fmt.Printf("\nFirst %d messages (raw FETCH responses):\n", len(fetchResult))
		for _, line := range fetchResult {
			fmt.Printf("  %s\n", line)
		}
	}

	// Logout
	if err := c.Logout(); err != nil {
		log.Fatalf("Logout failed: %v", err)
	}
	fmt.Println("\nLogged out")
}
