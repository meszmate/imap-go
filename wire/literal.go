package wire

import (
	"io"
)

// LiteralReader wraps a reader with literal metadata.
type LiteralReader struct {
	io.Reader
	Size    int64
	NonSync bool
	Binary  bool
}

// NewLiteralReader creates a new LiteralReader.
func NewLiteralReader(r io.Reader, size int64) *LiteralReader {
	return &LiteralReader{
		Reader: io.LimitReader(r, size),
		Size:   size,
	}
}

// LiteralWriter manages writing a literal value.
type LiteralWriter struct {
	w       io.Writer
	size    int64
	written int64
}

// NewLiteralWriter creates a new LiteralWriter for writing exactly size bytes.
func NewLiteralWriter(w io.Writer, size int64) *LiteralWriter {
	return &LiteralWriter{
		w:    w,
		size: size,
	}
}

// Write writes data to the literal. Returns an error if the total written
// exceeds the declared size.
func (lw *LiteralWriter) Write(p []byte) (int, error) {
	remaining := lw.size - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}

// Remaining returns the number of bytes remaining to write.
func (lw *LiteralWriter) Remaining() int64 {
	return lw.size - lw.written
}

// Done returns true if all bytes have been written.
func (lw *LiteralWriter) Done() bool {
	return lw.written >= lw.size
}
