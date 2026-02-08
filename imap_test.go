package imap

import (
	"strings"
	"testing"
	"time"
)

// --- ConnState tests ---

func TestConnState_String(t *testing.T) {
	tests := []struct {
		state ConnState
		want  string
	}{
		{ConnStateNotAuthenticated, "not authenticated"},
		{ConnStateAuthenticated, "authenticated"},
		{ConnStateSelected, "selected"},
		{ConnStateLogout, "logout"},
		{ConnState(99), "unknown(99)"},
		{ConnState(-1), "unknown(-1)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.state.String()
			if got != tt.want {
				t.Errorf("ConnState(%d).String() = %q, want %q", int(tt.state), got, tt.want)
			}
		})
	}
}

// --- NumKind tests ---

func TestNumKind_String(t *testing.T) {
	tests := []struct {
		kind NumKind
		want string
	}{
		{NumKindSeq, "seq"},
		{NumKindUID, "uid"},
		{NumKind(42), "unknown(42)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("NumKind(%d).String() = %q, want %q", int(tt.kind), got, tt.want)
			}
		})
	}
}

// --- Flag tests ---

func TestFlag_Values(t *testing.T) {
	tests := []struct {
		flag Flag
		want string
	}{
		{FlagSeen, "\\Seen"},
		{FlagAnswered, "\\Answered"},
		{FlagFlagged, "\\Flagged"},
		{FlagDeleted, "\\Deleted"},
		{FlagDraft, "\\Draft"},
		{FlagRecent, "\\Recent"},
		{FlagWildcard, "\\*"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.flag) != tt.want {
				t.Errorf("Flag = %q, want %q", tt.flag, tt.want)
			}
		})
	}
}

func TestFlag_CustomFlag(t *testing.T) {
	custom := Flag("$Important")
	if string(custom) != "$Important" {
		t.Errorf("custom flag = %q, want %q", custom, "$Important")
	}
}

// --- MailboxAttr tests ---

func TestMailboxAttr_Values(t *testing.T) {
	tests := []struct {
		attr MailboxAttr
		want string
	}{
		{MailboxAttrNoInferiors, "\\Noinferiors"},
		{MailboxAttrNoSelect, "\\Noselect"},
		{MailboxAttrMarked, "\\Marked"},
		{MailboxAttrUnmarked, "\\Unmarked"},
		{MailboxAttrHasChildren, "\\HasChildren"},
		{MailboxAttrHasNoChildren, "\\HasNoChildren"},
		{MailboxAttrNonExistent, "\\NonExistent"},
		{MailboxAttrSubscribed, "\\Subscribed"},
		{MailboxAttrRemote, "\\Remote"},
		{MailboxAttrAll, "\\All"},
		{MailboxAttrArchive, "\\Archive"},
		{MailboxAttrDrafts, "\\Drafts"},
		{MailboxAttrFlagged, "\\Flagged"},
		{MailboxAttrJunk, "\\Junk"},
		{MailboxAttrSent, "\\Sent"},
		{MailboxAttrTrash, "\\Trash"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.attr) != tt.want {
				t.Errorf("MailboxAttr = %q, want %q", tt.attr, tt.want)
			}
		})
	}
}

// --- Address tests ---

func TestAddress_String(t *testing.T) {
	tests := []struct {
		name string
		addr Address
		want string
	}{
		{
			"full address with name",
			Address{Name: "John Doe", Mailbox: "john", Host: "example.com"},
			"John Doe <john@example.com>",
		},
		{
			"address without name",
			Address{Mailbox: "john", Host: "example.com"},
			"john@example.com",
		},
		{
			"empty name",
			Address{Name: "", Mailbox: "alice", Host: "test.org"},
			"alice@test.org",
		},
		{
			"name with special chars",
			Address{Name: "Jane \"J\" Doe", Mailbox: "jane", Host: "example.com"},
			"Jane \"J\" Doe <jane@example.com>",
		},
		{
			"empty mailbox and host",
			Address{Name: "No Address", Mailbox: "", Host: ""},
			"No Address <@>",
		},
		{
			"all empty",
			Address{},
			"@",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.addr.String()
			if got != tt.want {
				t.Errorf("Address.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddress_StringPointer(t *testing.T) {
	addr := &Address{Name: "Test", Mailbox: "test", Host: "example.com"}
	want := "Test <test@example.com>"
	if got := addr.String(); got != want {
		t.Errorf("Address.String() = %q, want %q", got, want)
	}
}

// --- BodyStructure tests ---

func TestBodyStructure_IsMultipart(t *testing.T) {
	tests := []struct {
		name     string
		bs       BodyStructure
		want     bool
	}{
		{"multipart lower", BodyStructure{Type: "multipart", Subtype: "mixed"}, true},
		{"multipart upper", BodyStructure{Type: "MULTIPART", Subtype: "mixed"}, true},
		{"multipart mixed case", BodyStructure{Type: "Multipart", Subtype: "alternative"}, true},
		{"text plain", BodyStructure{Type: "text", Subtype: "plain"}, false},
		{"text html", BodyStructure{Type: "text", Subtype: "html"}, false},
		{"application", BodyStructure{Type: "application", Subtype: "pdf"}, false},
		{"image", BodyStructure{Type: "image", Subtype: "png"}, false},
		{"message rfc822", BodyStructure{Type: "message", Subtype: "rfc822"}, false},
		{"empty type", BodyStructure{Type: "", Subtype: ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bs.IsMultipart()
			if got != tt.want {
				t.Errorf("BodyStructure{Type: %q}.IsMultipart() = %v, want %v", tt.bs.Type, got, tt.want)
			}
		})
	}
}

func TestBodyStructure_IsMultipartWithChildren(t *testing.T) {
	bs := BodyStructure{
		Type:    "multipart",
		Subtype: "mixed",
		Children: []BodyStructure{
			{Type: "text", Subtype: "plain"},
			{Type: "text", Subtype: "html"},
		},
	}
	if !bs.IsMultipart() {
		t.Error("multipart with children should be multipart")
	}
	if bs.Children[0].IsMultipart() {
		t.Error("text/plain child should not be multipart")
	}
	if bs.Children[1].IsMultipart() {
		t.Error("text/html child should not be multipart")
	}
}

func TestBodyStructure_Fields(t *testing.T) {
	bs := BodyStructure{
		Type:        "text",
		Subtype:     "plain",
		Params:      map[string]string{"charset": "utf-8"},
		ID:          "<msg123@example.com>",
		Description: "A test message",
		Encoding:    "7bit",
		Size:        1024,
		Lines:       42,
		MD5:         "abc123",
		Disposition: "inline",
		DispositionParams: map[string]string{"filename": "test.txt"},
		Language:    []string{"en"},
		Location:    "http://example.com",
	}

	if bs.Type != "text" {
		t.Errorf("Type = %q, want %q", bs.Type, "text")
	}
	if bs.Subtype != "plain" {
		t.Errorf("Subtype = %q, want %q", bs.Subtype, "plain")
	}
	if bs.Params["charset"] != "utf-8" {
		t.Errorf("Params[charset] = %q, want %q", bs.Params["charset"], "utf-8")
	}
	if bs.ID != "<msg123@example.com>" {
		t.Errorf("ID = %q, want %q", bs.ID, "<msg123@example.com>")
	}
	if bs.Encoding != "7bit" {
		t.Errorf("Encoding = %q, want %q", bs.Encoding, "7bit")
	}
	if bs.Size != 1024 {
		t.Errorf("Size = %d, want %d", bs.Size, 1024)
	}
	if bs.Lines != 42 {
		t.Errorf("Lines = %d, want %d", bs.Lines, 42)
	}
	if bs.Disposition != "inline" {
		t.Errorf("Disposition = %q, want %q", bs.Disposition, "inline")
	}
	if len(bs.Language) != 1 || bs.Language[0] != "en" {
		t.Errorf("Language = %v, want [en]", bs.Language)
	}
}

func TestBodyStructure_EmbeddedMessage(t *testing.T) {
	bs := BodyStructure{
		Type:    "message",
		Subtype: "rfc822",
		Envelope: &Envelope{
			Subject: "Embedded subject",
		},
		BodyStructure: &BodyStructure{
			Type:    "text",
			Subtype: "plain",
		},
	}

	if bs.IsMultipart() {
		t.Error("message/rfc822 should not be multipart")
	}
	if bs.Envelope == nil {
		t.Fatal("Envelope should not be nil")
	}
	if bs.Envelope.Subject != "Embedded subject" {
		t.Errorf("Envelope.Subject = %q, want %q", bs.Envelope.Subject, "Embedded subject")
	}
	if bs.BodyStructure == nil {
		t.Fatal("embedded BodyStructure should not be nil")
	}
	if bs.BodyStructure.Type != "text" {
		t.Errorf("embedded Type = %q, want %q", bs.BodyStructure.Type, "text")
	}
}

// --- InternalDate tests ---

func TestInternalDate_String(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			"basic date",
			time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC),
			"15-Oct-2023 14:30:00 +0000",
		},
		{
			"january",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"01-Jan-2024 00:00:00 +0000",
		},
		{
			"with timezone offset",
			time.Date(2023, 6, 20, 10, 15, 30, 0, time.FixedZone("EST", -5*3600)),
			"20-Jun-2023 10:15:30 -0500",
		},
		{
			"positive timezone",
			time.Date(2023, 12, 25, 23, 59, 59, 0, time.FixedZone("IST", 5*3600+30*60)),
			"25-Dec-2023 23:59:59 +0530",
		},
		{
			"february leap year",
			time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			"29-Feb-2024 12:00:00 +0000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := InternalDate(tt.t)
			got := d.String()
			if got != tt.want {
				t.Errorf("InternalDate.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInternalDate_RoundTrip(t *testing.T) {
	// Parse a date string, convert to InternalDate, and verify the output matches
	original := "15-Oct-2023 14:30:00 +0000"
	parsed, err := time.Parse(InternalDateLayout, original)
	if err != nil {
		t.Fatalf("time.Parse(%q) error: %v", original, err)
	}
	d := InternalDate(parsed)
	got := d.String()
	if got != original {
		t.Errorf("round-trip: got %q, want %q", got, original)
	}
}

func TestInternalDateLayout(t *testing.T) {
	// Verify the layout constant is correct
	if InternalDateLayout != "02-Jan-2006 15:04:05 -0700" {
		t.Errorf("InternalDateLayout = %q, want %q", InternalDateLayout, "02-Jan-2006 15:04:05 -0700")
	}
}

// --- Envelope tests ---

func TestEnvelope_Fields(t *testing.T) {
	env := &Envelope{
		Date:      time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC),
		Subject:   "Test Subject",
		From:      []*Address{{Name: "Sender", Mailbox: "sender", Host: "example.com"}},
		To:        []*Address{{Name: "Recipient", Mailbox: "rcpt", Host: "example.com"}},
		Cc:        []*Address{},
		Bcc:       nil,
		InReplyTo: "<reply123@example.com>",
		MessageID: "<msg456@example.com>",
	}

	if env.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", env.Subject, "Test Subject")
	}
	if len(env.From) != 1 {
		t.Fatalf("From length = %d, want 1", len(env.From))
	}
	if env.From[0].String() != "Sender <sender@example.com>" {
		t.Errorf("From[0].String() = %q, want %q", env.From[0].String(), "Sender <sender@example.com>")
	}
	if len(env.To) != 1 {
		t.Fatalf("To length = %d, want 1", len(env.To))
	}
	if env.InReplyTo != "<reply123@example.com>" {
		t.Errorf("InReplyTo = %q", env.InReplyTo)
	}
	if env.MessageID != "<msg456@example.com>" {
		t.Errorf("MessageID = %q", env.MessageID)
	}
}

func TestEnvelope_MultipleRecipients(t *testing.T) {
	env := &Envelope{
		To: []*Address{
			{Name: "Alice", Mailbox: "alice", Host: "example.com"},
			{Name: "Bob", Mailbox: "bob", Host: "example.com"},
			{Mailbox: "charlie", Host: "example.com"},
		},
	}
	if len(env.To) != 3 {
		t.Fatalf("To length = %d, want 3", len(env.To))
	}
	if env.To[0].String() != "Alice <alice@example.com>" {
		t.Errorf("To[0] = %q", env.To[0].String())
	}
	if env.To[2].String() != "charlie@example.com" {
		t.Errorf("To[2] = %q", env.To[2].String())
	}
}

// --- BodySectionName tests ---

func TestBodySectionName_Fields(t *testing.T) {
	bsn := BodySectionName{
		Specifier: "HEADER.FIELDS",
		Part:      []int{1, 2},
		Fields:    []string{"From", "To", "Subject"},
		NotFields: false,
		Peek:      true,
		Partial: &SectionPartial{
			Offset: 0,
			Count:  100,
		},
	}

	if bsn.Specifier != "HEADER.FIELDS" {
		t.Errorf("Specifier = %q", bsn.Specifier)
	}
	if len(bsn.Part) != 2 || bsn.Part[0] != 1 || bsn.Part[1] != 2 {
		t.Errorf("Part = %v, want [1, 2]", bsn.Part)
	}
	if len(bsn.Fields) != 3 {
		t.Errorf("Fields length = %d, want 3", len(bsn.Fields))
	}
	if bsn.NotFields {
		t.Error("NotFields should be false")
	}
	if !bsn.Peek {
		t.Error("Peek should be true")
	}
	if bsn.Partial == nil {
		t.Fatal("Partial should not be nil")
	}
	if bsn.Partial.Offset != 0 || bsn.Partial.Count != 100 {
		t.Errorf("Partial = %+v, want {0, 100}", bsn.Partial)
	}
}

func TestBodySectionName_NotFields(t *testing.T) {
	bsn := BodySectionName{
		Specifier: "HEADER.FIELDS.NOT",
		Fields:    []string{"X-Spam"},
		NotFields: true,
	}
	if !bsn.NotFields {
		t.Error("NotFields should be true")
	}
}

// --- SectionPartial tests ---

func TestSectionPartial(t *testing.T) {
	sp := SectionPartial{Offset: 10, Count: 200}
	if sp.Offset != 10 {
		t.Errorf("Offset = %d, want 10", sp.Offset)
	}
	if sp.Count != 200 {
		t.Errorf("Count = %d, want 200", sp.Count)
	}
}

// --- LiteralReader tests ---

func TestLiteralReader(t *testing.T) {
	r := strings.NewReader("hello world")
	lr := LiteralReader{
		Reader: r,
		Size:   11,
	}
	if lr.Size != 11 {
		t.Errorf("Size = %d, want 11", lr.Size)
	}
	buf := make([]byte, 5)
	n, err := lr.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 5 {
		t.Errorf("Read n = %d, want 5", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Read data = %q, want %q", buf, "hello")
	}
}

// --- CreateOptions tests ---

func TestCreateOptions(t *testing.T) {
	opts := CreateOptions{
		SpecialUse: MailboxAttrDrafts,
	}
	if opts.SpecialUse != MailboxAttrDrafts {
		t.Errorf("SpecialUse = %q, want %q", opts.SpecialUse, MailboxAttrDrafts)
	}
}

// --- UID / SeqNum type tests ---

func TestUID_Type(t *testing.T) {
	var uid UID = 12345
	if uint32(uid) != 12345 {
		t.Errorf("UID = %d, want 12345", uid)
	}
}

func TestSeqNum_Type(t *testing.T) {
	var seq SeqNum = 42
	if uint32(seq) != 42 {
		t.Errorf("SeqNum = %d, want 42", seq)
	}
}
