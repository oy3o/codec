package codec

import (
	"io"
)

// PeekableReader is a reader that allows peeking ahead at the underlying data stream.
type PeekableReader struct {
	R io.Reader // The underlying reader.
	B []byte    // The buffer for peeked data.
}

// PeekReader returns a PeekableReader. If the given reader is already a
// PeekableReader, it is returned directly.
func PeekReader(r io.Reader) *PeekableReader {
	if pr, ok := r.(*PeekableReader); ok {
		return pr
	}
	return &PeekableReader{R: r}
}

// Peek returns the next n bytes without advancing the reader.
func (r *PeekableReader) Peek(n int) ([]byte, error) {
	// If the buffer already contains enough bytes, return them.
	if len(r.B) >= n {
		return r.B[:n], nil
	}

	// Read more data from the underlying reader to satisfy the peek request.
	i := len(r.B)
	needs := n - i
	r.B = append(r.B, make([]byte, needs)...)

	var err error
	for i < n {
		read, er := r.R.Read(r.B[i:])
		i += read
		if er != nil {
			err = er
			break
		}
	}
	// Trim the buffer to the actual number of bytes read.
	if i != n {
		r.B = r.B[:i]
	}
	return r.B, err
}

// Close closes the underlying reader if it implements io.Closer.
func (r *PeekableReader) Close() error {
	if c, ok := r.R.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

// Read reads data into p. It first reads from the peeked buffer and then
// from the underlying reader if necessary.
func (r *PeekableReader) Read(p []byte) (n int, err error) {
	// Copy data from the peek buffer first.
	n = copy(p, r.B)
	if len(p) <= len(r.B) {
		// If p is smaller than the buffer, slice the buffer.
		r.B = r.B[n:]
		return n, nil
	}
	// If p is larger, the entire buffer is consumed.
	r.B = nil
	// Read the rest of the data from the underlying reader.
	read, err := r.R.Read(p[n:])
	n += read
	return n, err
}

// WriteTo writes data to w. It first writes the peeked buffer and then
// copies from the underlying reader.
func (r *PeekableReader) WriteTo(w io.Writer) (n int64, err error) {
	// Write the buffered data first.
	if len(r.B) > 0 {
		written, err := w.Write(r.B)
		n = int64(written)
		r.B = r.B[written:]
		if err != nil {
			return n, err
		}
		// If not all buffered data could be written, return ErrShortWrite.
		if len(r.B) > 0 {
			return n, io.ErrShortWrite
		}
	}

	// Use optimized WriteTo if the reader supports it.
	if wt, ok := r.R.(io.WriterTo); ok {
		return wt.WriteTo(w)
	}
	// Use optimized ReadFrom if the writer supports it.
	if rf, ok := w.(io.ReaderFrom); ok {
		return rf.ReadFrom(r.R)
	}

	// Use a buffer from the pool for manual copying.
	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	// Manually read from the reader and write to the writer.
	for {
		read, er := r.R.Read(buf)
		if read > 0 {
			written, ew := w.Write(buf[0:read])
			n += int64(written)
			if ew != nil {
				err = ew
				break
			}
			if read != written {
				err = io.ErrShortWrite
				// Buffer the unwritten data.
				r.B = make([]byte, read-written)
				copy(r.B, buf[written:read])
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}

	return n, err
}
