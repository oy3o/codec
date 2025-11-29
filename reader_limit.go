package codec

import "io"

type LimitedReader struct {
	*io.LimitedReader
}

func LimitReader(r io.Reader, n int64) reader {
	return &LimitedReader{&io.LimitedReader{R: r, N: n}}
}

// Close closes the underlying reader if it implements io.Closer.
func (r *LimitedReader) Close() error {
	if c, ok := r.R.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

// WriteTo implements the io.WriterTo interface, providing an optimized path for io.Copy.
// It reads from the underlying reader, writes to the destination writer
func (r *LimitedReader) WriteTo(w io.Writer) (n int64, err error) {
	// Check if the destination writer implements io.ReaderFrom for a more efficient copy.
	if rf, ok := w.(io.ReaderFrom); ok {
		return rf.ReadFrom(r.LimitedReader)
	}

	// Fallback to a generic path using a buffer.
	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	for {
		// 1. Read a chunk from the underlying reader.
		read, er := r.Read(buf)
		if read > 0 {
			// 2. Write the chunk to the destination writer.
			written, ew := w.Write(buf[0:read])

			// 3. Report the number of bytes successfully written.
			n += int64(written)
			if ew != nil {
				err = ew
				break
			}
			// If we couldn't write the whole chunk, it's a short write.
			if read != written {
				err = io.ErrShortWrite
				break
			}
		}
		// If read fails, we are done with the loop.
		if er != nil {
			if er != io.EOF { // EOF is the signal for a successful end.
				err = er
			}
			break
		}
	}

	return n, err
}
