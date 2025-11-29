package codec

import "io"

// BytesWriter is an io.Writer that writes to a pre-allocated byte slice.
// It will not grow the slice's capacity. If a write exceeds the available space,
// it writes as much as it can and returns io.ErrShortWrite.
type BytesWriter struct {
	B []byte // destination slice
	N int    // current write position
}

// NewBytesWriter creates a new BytesWriter.
func NewBytesWriter(p []byte) *BytesWriter {
	return &BytesWriter{B: p[:cap(p)]}
}

// Close closes the underlying writer if it implements io.Closer.
func (w *BytesWriter) Close() error {
	return nil
}

// Write implements the io.Writer interface.
func (w *BytesWriter) Write(p []byte) (int, error) {
	if w.N >= len(w.B) {
		return 0, io.ErrShortWrite
	}
	n := copy(w.B[w.N:], p)
	w.N += n
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

func (w *BytesWriter) WriteZeros(n int) (int, error) {
	if w.N >= len(w.B) {
		return 0, io.ErrShortWrite
	}
	if n <= BUFFER_SIZE {
		written := copy(w.B[w.N:], empty[:n])
		w.N += written
		if written < n {
			return written, io.ErrShortWrite
		}
		return written, nil
	} else {
		// Fallback to the efficient io.CopyN for larger padding.
		written, err := io.CopyN(w, Zero, int64(n))
		return int(written), err
	}
}

// WriteString implements the io.StringWriter interface for efficiency.
func (w *BytesWriter) WriteString(s string) (int, error) {
	if w.N >= len(w.B) {
		return 0, io.ErrShortWrite
	}
	n := copy(w.B[w.N:], s)
	w.N += n
	if n < len(s) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// WriteByte implements the io.ByteWriter interface for efficiency.
func (w *BytesWriter) WriteByte(c byte) error {
	if w.N >= len(w.B) {
		return io.ErrShortWrite
	}
	w.B[w.N] = c
	w.N++
	return nil
}

// ReadFrom implements the io.ReaderFrom interface and reads data from r until EOF or an error occurs.
func (w *BytesWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.N >= len(w.B) {
		return 0, io.ErrShortWrite
	}
	n, err := r.Read(w.B[w.N:])
	if n < 0 {
		return 0, ErrInvalidWrite
	}
	w.N += n
	if err == io.EOF {
		return int64(n), nil
	}
	return int64(n), err
}

// Flush do nothing
func (w *BytesWriter) Flush() error { return nil }

// Reset allows the underlying byte slice to be reused.
func (w *BytesWriter) Reset() { w.N = 0 }

// Len returns the number of bytes written.
func (w *BytesWriter) Len() int { return w.N }

// Size returns the capacity of the underlying byte slice.
func (w *BytesWriter) Size() int { return len(w.B) }

// Available returns the number of bytes available for writing.
func (w *BytesWriter) Available() int { return len(w.B) - w.N }

// Bytes returns a slice view of the written data.
func (w *BytesWriter) Bytes() []byte { return w.B[:w.N] }
