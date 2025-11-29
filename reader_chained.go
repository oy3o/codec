package codec

import (
	"fmt"
	"io"
)

// ChainedReaderCallback is the function type for an action to be executed
// after the main data stream has been fully consumed. It receives the
// underlying reader to continue reading from the stream (e.g., to process a trailer).
type ChainedReaderCallback func(trailerReader io.Reader) error

// ChainedReader is a reader that wraps an underlying stream. It reads a predefined
// number of bytes (the main stream) and then executes a callback action on the
// remainder of the stream. This is useful for handling data formats where a
// payload is followed by a trailer or metadata.
type ChainedReader struct {
	U io.Reader             // U is the underlying raw stream (e.g., a net.Conn or os.File).
	R *io.LimitedReader     // R limits reading to the length of the main data stream.
	C ChainedReaderCallback // C is the callback to execute after the main stream is read.
	E bool                  // E (executed) is a flag to ensure the callback runs only once.
	N int64                 // N stores the original length of the main stream for seeking purposes.
}

// ChainReader creates a new ChainedReader which also satisfies io.ReadCloser.
// If the provided reader also implements io.Seeker, the returned value will
// support seeking as well.
//
// reader: The underlying io.Reader.
// n: The number of bytes in the main data stream.
// callback: The function to execute after n bytes have been read.
func ChainReader(reader io.Reader, n int64, callback ChainedReaderCallback) reader {
	cr := &ChainedReader{
		U: reader,
		R: &io.LimitedReader{R: reader, N: n},
		C: callback,
		N: n,
	}
	// Progressively enhance with Seeker capability if the underlying reader supports it.
	if seeker, ok := reader.(io.Seeker); ok {
		return &ChainedReadSeeker{cr, seeker}
	}
	return cr
}

// Read implements the io.Reader interface. It reads from the main stream.
// When the main stream is exhausted, it executes the chained callback
// before returning io.EOF.
func (r *ChainedReader) Read(p []byte) (n int, err error) {
	// If the callback has run and the stream is exhausted, consistently return EOF.
	if r.E && r.R.N <= 0 {
		return 0, io.EOF
	}

	n, err = r.R.Read(p)

	// The main stream is exhausted when the limited reader is drained (r.R.N == 0)
	// or when the underlying reader returns EOF.
	if r.R.N == 0 || err == io.EOF { // handle case where EOF and data are returned together
		if r.E { // Safety check to ensure the callback is not run again.
			return n, io.EOF
		}
		r.E = true // Mark the callback as executed.

		if r.C != nil {
			actionErr := r.C(r.U)
			if actionErr != nil {
				// If the callback fails, its error is more significant than the EOF.
				return n, fmt.Errorf("chained action failed after reading main stream: %w", actionErr)
			}
		}

		// If the original error was not nil (e.g., EOF), return it.
		// If the original error was nil but we reached the limit, return EOF.
		if err == nil {
			return n, io.EOF
		}
		return n, err
	}

	return n, err
}

// Close closes the underlying reader if it implements io.Closer.
func (r *ChainedReader) Close() error {
	if closer, ok := r.U.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// WriteTo implements io.WriterTo for efficient copying, like in io.Copy.
func (r *ChainedReader) WriteTo(w io.Writer) (n int64, err error) {
	// If already exhausted, there's nothing to write.
	if r.E && r.R.N <= 0 {
		return 0, nil
	}

	// Copy exactly the remaining bytes of the main stream.
	written, err := io.CopyN(w, r.R, r.R.N)
	n += written
	if err != nil {
		return n, err
	}

	// If the callback has already been run by a previous operation, we're done.
	if r.E {
		return n, nil
	}

	// Mark as executed and run the callback.
	r.E = true
	if r.C != nil {
		actionErr := r.C(r.U)
		if actionErr != nil {
			return n, fmt.Errorf("chained action failed after writing main stream: %w", actionErr)
		}
	}
	return n, nil
}

// ChainedReadSeeker embeds a ChainedReader to add seeking capability.
// It is returned by ChainReader when the underlying reader implements io.Seeker.
type ChainedReadSeeker struct {
	*ChainedReader
	S io.Seeker
}

// Seek implements the io.Seeker interface. It adjusts the position within
// the stream and resets the state of the ChainedReader accordingly.
func (r *ChainedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	// Perform the seek on the underlying reader.
	n, err := r.S.Seek(offset, whence)
	if err != nil {
		return n, err
	}

	// Recalculate the remaining bytes for the LimitedReader based on the new position.
	// r.N is the total size of the main stream.
	// n is the new offset from the start.
	r.R.N = r.N - n

	// If we seek back into the main stream, the chained action has not been
	// executed yet for this new pass, so we must reset the flag.
	r.E = false

	return n, nil
}
