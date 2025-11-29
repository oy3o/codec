package codec

import (
	"fmt"
	"io"
)

// forwardSeekCloser wraps an io.ReadCloser, adding a forward-only Seek capability
// to satisfy the io.ReadSeekCloser interface. It simulates seeking by reading
// and discarding data.
type forwardSeekCloser struct {
	r      io.ReadCloser
	offset int64
}

// ForwardSeekCloser wraps an io.ReadCloser to make it a forward-only io.ReadSeekCloser.
// If the provided reader already implements io.ReadSeekCloser, it is returned directly.
func ForwardSeekCloser(r io.ReadCloser) io.ReadSeekCloser {
	if r == nil {
		panic("codec: ForwardSeekCloser called with a nil io.ReadCloser")
	}
	if seeker, ok := r.(io.ReadSeekCloser); ok {
		return seeker
	}
	return &forwardSeekCloser{r: r}
}

// Read implements the io.Reader interface.
func (s *forwardSeekCloser) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.offset += int64(n)
	return n, err
}

// Close implements the io.Closer interface.
func (s *forwardSeekCloser) Close() error {
	return s.r.Close()
}

// Seek provides forward-only seeking.
// It supports io.SeekCurrent and io.SeekStart, but will return an error
// for any backward seek attempts.
func (s *forwardSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var forwardOffset int64

	switch whence {
	case io.SeekCurrent:
		forwardOffset = offset
	case io.SeekStart:
		if offset < s.offset {
			return s.offset, fmt.Errorf("%w: cannot seek from start to %d (current: %d)", ErrUnsupportedNegativeSeek, offset, s.offset)
		}
		forwardOffset = offset - s.offset
	default:
		return s.offset, fmt.Errorf("%w: value %d is not supported", ErrInvalidWhence, whence)
	}

	if forwardOffset < 0 {
		return s.offset, ErrUnsupportedNegativeSeek
	}
	if forwardOffset == 0 {
		return s.offset, nil
	}

	// Discard data efficiently to perform the "seek".
	written, err := Discard(s.r, forwardOffset)
	s.offset += written
	return s.offset, err
}

// forwardSeeker wraps an io.Reader, adding a forward-only Seek capability
// to satisfy the io.ReadSeeker interface. It simulates seeking by reading
// and discarding data.
type forwardSeeker struct {
	r      io.Reader
	offset int64
}

// ForwardSeeker wraps an io.Reader to make it a forward-only io.ReadSeeker.
// If the provided reader already implements io.ReadSeeker, it is returned directly.
func ForwardSeeker(r io.Reader) io.ReadSeeker {
	if r == nil {
		panic("codec: ForwardSeeker called with a nil io.Reader")
	}
	if seeker, ok := r.(io.ReadSeeker); ok {
		return seeker
	}
	return &forwardSeeker{r: r}
}

// Read implements the io.Reader interface.
func (s *forwardSeeker) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.offset += int64(n)
	return n, err
}

// Seek provides forward-only seeking.
// It supports io.SeekCurrent and io.SeekStart, but will return an error
// for any backward seek attempts.
func (s *forwardSeeker) Seek(offset int64, whence int) (int64, error) {
	var skip int64

	switch whence {
	case io.SeekCurrent:
		skip = offset
	case io.SeekStart:
		if offset < s.offset {
			return s.offset, fmt.Errorf("%w: cannot seek from start to %d (current: %d)", ErrUnsupportedNegativeSeek, offset, s.offset)
		}
		skip = offset - s.offset
	default:
		return s.offset, fmt.Errorf("%w: value %d is not supported", ErrInvalidWhence, whence)
	}

	if skip < 0 {
		return s.offset, ErrUnsupportedNegativeSeek
	}
	if skip == 0 {
		return s.offset, nil
	}

	// Discard data efficiently to perform the "seek".
	written, err := Discard(s.r, skip)
	s.offset += written
	return s.offset, err
}
