package wire

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// ==================== LiteralReader ====================

func TestNewLiteralReader(t *testing.T) {
	data := "hello world and more"
	r := NewLiteralReader(strings.NewReader(data), 5)

	if r.Size != 5 {
		t.Errorf("Size = %d, want 5", r.Size)
	}
	if r.NonSync != false {
		t.Errorf("NonSync = %v, want false", r.NonSync)
	}
	if r.Binary != false {
		t.Errorf("Binary = %v, want false", r.Binary)
	}
}

func TestLiteralReaderLimitsRead(t *testing.T) {
	data := "hello world and more"
	r := NewLiteralReader(strings.NewReader(data), 5)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("Read = %q, want %q", got, "hello")
	}
}

func TestLiteralReaderZeroSize(t *testing.T) {
	r := NewLiteralReader(strings.NewReader("anything"), 0)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Read = %q, want empty", got)
	}
}

func TestLiteralReaderExactSize(t *testing.T) {
	data := "12345"
	r := NewLiteralReader(strings.NewReader(data), 5)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "12345" {
		t.Errorf("Read = %q, want %q", got, "12345")
	}
}

func TestLiteralReaderSizeLargerThanData(t *testing.T) {
	data := "short"
	r := NewLiteralReader(strings.NewReader(data), 100)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	// Should only return what's available (limited by underlying reader)
	if string(got) != "short" {
		t.Errorf("Read = %q, want %q", got, "short")
	}
}

func TestLiteralReaderNonSyncAndBinaryFlags(t *testing.T) {
	r := NewLiteralReader(strings.NewReader("data"), 4)
	r.NonSync = true
	r.Binary = true

	if !r.NonSync {
		t.Error("NonSync should be true")
	}
	if !r.Binary {
		t.Error("Binary should be true")
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "data" {
		t.Errorf("Read = %q, want %q", got, "data")
	}
}

func TestLiteralReaderMultipleReads(t *testing.T) {
	data := "abcdefghij"
	r := NewLiteralReader(strings.NewReader(data), 8)

	buf := make([]byte, 3)

	// First read: "abc"
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || string(buf[:n]) != "abc" {
		t.Errorf("first read: n=%d, data=%q", n, buf[:n])
	}

	// Second read: "def"
	n, err = r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || string(buf[:n]) != "def" {
		t.Errorf("second read: n=%d, data=%q", n, buf[:n])
	}

	// Third read: "gh" (only 2 remaining of the 8 limit)
	n, err = r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if n != 2 || string(buf[:n]) != "gh" {
		t.Errorf("third read: n=%d, data=%q", n, buf[:n])
	}

	// Fourth read: EOF
	n, err = r.Read(buf)
	if err != io.EOF {
		t.Errorf("expected EOF, got err=%v, n=%d", err, n)
	}
}

func TestLiteralReaderBinaryData(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	r := NewLiteralReader(bytes.NewReader(data), int64(len(data)))

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read = %v, want %v", got, data)
	}
}

// ==================== LiteralWriter ====================

func TestNewLiteralWriter(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 10)

	if lw.Remaining() != 10 {
		t.Errorf("Remaining() = %d, want 10", lw.Remaining())
	}
	if lw.Done() {
		t.Error("Done() should be false initially")
	}
}

func TestLiteralWriterWriteExact(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 5)

	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("Write() = %d, want 5", n)
	}
	if buf.String() != "hello" {
		t.Errorf("written = %q, want %q", buf.String(), "hello")
	}
	if lw.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", lw.Remaining())
	}
	if !lw.Done() {
		t.Error("Done() should be true")
	}
}

func TestLiteralWriterWriteOverflow(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 5)

	// Write more than the declared size
	n, err := lw.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	// Should truncate to 5 bytes
	if n != 5 {
		t.Errorf("Write() = %d, want 5", n)
	}
	if buf.String() != "hello" {
		t.Errorf("written = %q, want %q", buf.String(), "hello")
	}
	if !lw.Done() {
		t.Error("Done() should be true after overflow")
	}

	// Additional writes should write 0 bytes
	n2, err := lw.Write([]byte("more"))
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("second Write() = %d, want 0", n2)
	}
}

func TestLiteralWriterIncrementalWrites(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 10)

	writes := []string{"hel", "lo ", "wor", "ld"}
	for _, w := range writes {
		_, err := lw.Write([]byte(w))
		if err != nil {
			t.Fatal(err)
		}
	}

	if buf.String() != "hello worl" {
		t.Errorf("written = %q, want %q", buf.String(), "hello worl")
	}
	if lw.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", lw.Remaining())
	}
	if !lw.Done() {
		t.Error("Done() should be true")
	}
}

func TestLiteralWriterZeroSize(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 0)

	if !lw.Done() {
		t.Error("Done() should be true for zero size")
	}
	if lw.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", lw.Remaining())
	}

	n, err := lw.Write([]byte("anything"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("Write() to zero-size writer = %d, want 0", n)
	}
}

func TestLiteralWriterRemaining(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 10)

	if lw.Remaining() != 10 {
		t.Errorf("initial Remaining() = %d, want 10", lw.Remaining())
	}

	_, _ = lw.Write([]byte("abc"))
	if lw.Remaining() != 7 {
		t.Errorf("after 3 bytes Remaining() = %d, want 7", lw.Remaining())
	}

	_, _ = lw.Write([]byte("defgh"))
	if lw.Remaining() != 2 {
		t.Errorf("after 8 bytes Remaining() = %d, want 2", lw.Remaining())
	}

	_, _ = lw.Write([]byte("ij"))
	if lw.Remaining() != 0 {
		t.Errorf("after 10 bytes Remaining() = %d, want 0", lw.Remaining())
	}
}

func TestLiteralWriterDone(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 3)

	if lw.Done() {
		t.Error("Done() should be false before any writes")
	}

	_, _ = lw.Write([]byte("ab"))
	if lw.Done() {
		t.Error("Done() should be false after partial write")
	}

	_, _ = lw.Write([]byte("c"))
	if !lw.Done() {
		t.Error("Done() should be true after writing all bytes")
	}
}

func TestLiteralWriterBinaryData(t *testing.T) {
	var buf bytes.Buffer
	data := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	lw := NewLiteralWriter(&buf, int64(len(data)))

	n, err := lw.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("written = %v, want %v", buf.Bytes(), data)
	}
}

func TestLiteralWriterPartialOverflow(t *testing.T) {
	// Write exactly at the boundary, then try to write more
	var buf bytes.Buffer
	lw := NewLiteralWriter(&buf, 5)

	n1, _ := lw.Write([]byte("abc")) // 3 bytes
	if n1 != 3 {
		t.Errorf("first Write = %d, want 3", n1)
	}

	n2, _ := lw.Write([]byte("defgh")) // attempt 5, only 2 should go through
	if n2 != 2 {
		t.Errorf("second Write = %d, want 2", n2)
	}

	if buf.String() != "abcde" {
		t.Errorf("total written = %q, want %q", buf.String(), "abcde")
	}
}

// ==================== LiteralReader and LiteralWriter integration ====================

func TestLiteralReaderWriterIntegration(t *testing.T) {
	// Write data through LiteralWriter, then read it back through LiteralReader
	var buf bytes.Buffer
	data := "integration test data"
	size := int64(len(data))

	// Write
	lw := NewLiteralWriter(&buf, size)
	n, err := lw.Write([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Fatalf("Write = %d, want %d", n, len(data))
	}

	// Read
	lr := NewLiteralReader(&buf, size)
	got, err := io.ReadAll(lr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != data {
		t.Errorf("round-trip: got %q, want %q", got, data)
	}
}
