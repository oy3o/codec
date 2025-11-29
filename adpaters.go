package codec

import (
	"bufio"
	"bytes"
	"io"
)

type (
	bytesReaderAdapter       struct{ *bytes.Reader }
	bytesBufferWriterAdapter struct{ *bytes.Buffer }
	bytesBufferReaderAdapter struct {
		*bytes.Buffer
		pos int64
	}
	bufioWriterAdapter struct{ *bufio.Writer }
	bufioReaderAdapter struct {
		*bufio.Reader
		seeker io.ReadSeeker
		pos    int64
	}
)

func (r *bytesReaderAdapter) Close() error       { return nil }
func (r *bufioReaderAdapter) Close() error       { return nil }
func (w *bufioWriterAdapter) Close() error       { return nil }
func (r *bytesBufferReaderAdapter) Close() error { return nil }
func (w *bytesBufferWriterAdapter) Close() error { return nil }
func (w *bytesBufferWriterAdapter) Flush() error { return nil }
func (w *bytesBufferWriterAdapter) Size() int    { return w.Available() }
func (r *bytesBufferReaderAdapter) Size() int    { return r.Len() }
func (r *bytesReaderAdapter) Size() int          { return int(r.Reader.Size()) }

// Read reads from the underlying buffer and updates the internal pos.
func (r *bytesBufferReaderAdapter) Read(p []byte) (n int, err error) {
	n, err = r.Buffer.Read(p)
	r.pos += int64(n)
	return n, err
}

// ReadByte reads a single byte from the buffer and updates the pos.
func (r *bytesBufferReaderAdapter) ReadByte() (byte, error) {
	b, err := r.Buffer.ReadByte()
	if err == nil {
		r.pos++
	}
	return b, err
}

// WriteTo writes the remaining buffer content to a writer and updates the pos.
func (r *bytesBufferReaderAdapter) WriteTo(w io.Writer) (n int64, err error) {
	n, err = r.Buffer.WriteTo(w)
	r.pos += n
	return n, err
}

// Seek performs a forward-only seek by discarding bytes from the buffer.
// It does not support seeking from the end (io.SeekEnd) or seeking backwards.
func (r *bytesBufferReaderAdapter) Seek(pos int64, whence int) (int64, error) {
	var target int64

	switch whence {
	case io.SeekStart:
		target = pos
	case io.SeekCurrent:
		target = r.pos + pos
	case io.SeekEnd:
		return r.pos, ErrInvalidWhence
	default:
		return r.pos, ErrInvalidWhence
	}

	// Calculate how many bytes to move forward.
	skip := target - r.pos

	if skip < 0 {
		return r.pos, ErrUnsupportedNegativeSeek
	}

	if skip == 0 {
		return r.pos, nil
	}

	// Discard bytes efficiently.
	// We can only discard as many bytes as are left in the buffer.
	if skip > int64(r.Buffer.Len()) {
		skip = int64(r.Buffer.Len())
	}

	// The Next method advances the buffer's read pointer.
	r.Buffer.Next(int(skip))

	// Update our absolute position.
	r.pos += skip

	return r.pos, nil
}

// Read reads data into p, updating the stream position.
func (b *bufioReaderAdapter) Read(p []byte) (n int, err error) {
	n, err = b.Reader.Read(p)
	b.pos += int64(n)
	return n, err
}

// ReadByte reads a single byte, updating the stream position.
func (b *bufioReaderAdapter) ReadByte() (c byte, err error) {
	c, err = b.Reader.ReadByte()
	if err == nil {
		b.pos++
	}
	return c, err
}

// WriteTo writes data to w, updating the stream position.
func (b *bufioReaderAdapter) WriteTo(w io.Writer) (n int64, err error) {
	n, err = b.Reader.WriteTo(w)
	b.pos += n
	return n, err
}

// Size returns the size of the underlying buffer.
func (b *bufioReaderAdapter) Size() int {
	return b.Reader.Size()
}

// Seek implements the io.Seeker interface and correctly handles the internal buffer of bufio.Reader.
// This is the standard pattern for implementing Seek on a buffered reader.
func (b *bufioReaderAdapter) Seek(offset int64, whence int) (int64, error) {
	// Calculate the absolute target position (target) based on whence.
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = b.pos + offset
	case io.SeekEnd:
		if b.seeker == nil {
			return b.pos, ErrInvalidWhence
		}
		endPos, err := b.seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		target = endPos + offset
	default:
		return 0, ErrInvalidWhence
	}

	// if target is within the buffer, just discard bytes.
	if b.pos <= target && target < b.pos+int64(b.Reader.Buffered()) {
		skip := int(target - b.pos)
		n, err := b.Reader.Discard(skip)
		b.pos += int64(n)
		return b.pos, err
	}

	// Case 1: The underlying stream is seekable. This is the most flexible case.
	if b.seeker != nil {
		// Target is outside the buffer, so we must seek the underlying stream
		// and reset the buffer.
		newPos, err := b.seeker.Seek(target, io.SeekStart)
		if err != nil {
			return 0, err
		}
		b.Reader.Reset(b.seeker)
		b.pos = newPos
		return newPos, nil
	}

	// Case 2: The underlying stream is not seekable. Seeking is very limited.
	if target < b.pos {
		return b.pos, ErrUnsupportedNegativeSeek
	}
	_, err := Discard(b, target-b.pos)
	return b.pos, err
}
