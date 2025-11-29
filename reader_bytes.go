package codec

import "io"

// BytesReader is an io.Reader that read from a pre-allocated byte slice.
type BytesReader struct {
	B []byte // destination slice
	N int    // current read position
}

// NewBytesReader creates a new BytesReader.
func NewBytesReader(b []byte) *BytesReader {
	return &BytesReader{B: b}
}

// Close closes the underlying reader if it implements io.Closer.
func (r *BytesReader) Close() error {
	return nil
}

// Read implements the [io.Reader] interface.
func (r *BytesReader) Read(p []byte) (int, error) {
	if r.N >= len(r.B) {
		return 0, io.EOF
	}
	n := copy(p, r.B[r.N:])
	r.N += n
	return n, nil
}

// ReadByte implements the [io.ByteReader] interface.
func (r *BytesReader) ReadByte() (byte, error) {
	if r.N >= len(r.B) {
		return 0, io.EOF
	}
	b := r.B[r.N]
	r.N++
	return b, nil
}

// WriteTo implements the [io.WriterTo] interface for efficiency.
func (r *BytesReader) WriteTo(w io.Writer) (int64, error) {
	if r.N >= len(r.B) {
		return 0, nil
	}

	b := r.B[r.N:]
	n, err := w.Write(b)
	if n > len(r.B) {
		return int64(n), ErrInvalidRead
	}
	r.N += n
	if err != nil {
		return int64(n), err
	}

	return int64(n), nil
}

// Seek implements the [io.Seeker] interface.
func (r *BytesReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = int64(r.N) + offset
	case io.SeekEnd:
		abs = int64(len(r.B)) + offset
	default:
		return 0, ErrInvalidWhence
	}

	if abs < 0 {
		return 0, ErrInvalidSeek
	}

	r.N = int(abs)
	return abs, nil
}

// Reset allows the underlying byte slice to be reused.
func (w *BytesReader) Reset() {
	w.N = 0
}

// Len returns the number of bytes read.
func (w *BytesReader) Len() int {
	return w.N
}

// Size returns the size of the underlying byte slice.
func (r *BytesReader) Size() int {
	return len(r.B)
}

// Available returns the number of bytes available for reading.
func (r *BytesReader) Available() int {
	length := len(r.B) - r.N
	if length <= 0 {
		return 0
	}
	return length
}
