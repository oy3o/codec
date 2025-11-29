package codec

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

// Zero is an io.Reader that reads an infinite stream of zero bytes.
var Zero io.Reader = zero{}

type zero struct{}

func (z zero) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type reader interface {
	io.Reader
	io.WriterTo
	io.Closer
}

type ReaderPro interface {
	reader
	io.ByteReader
	io.Seeker
	Size() int
}

// Reader provides a buffered reader that simplifies reading binary data.
// It wraps bufio.Reader and tracks the first error. Subsequent reads become no-ops.
type Reader struct {
	r     ReaderPro
	count int64 // total bytes read
	err   error // first error encountered.
	order binary.ByteOrder
}

var _ ReaderPro = (*Reader)(nil)

// NewReaderSize creates a new Reader with a specified buffer size.
func NewReaderSize(r io.Reader, size int) (*Reader, error) {
	if r == nil {
		return nil, ErrNilIO
	}

	switch reader := r.(type) {
	// Reuse the underlying buffer if it's already a compatible Reader.
	case *Reader:
		if reader.r.Size() >= size {
			return &Reader{r: reader.r, order: Order}, nil
		}

	// prevent unpredictable double-buffering.
	case *bufio.Reader:
		if reader.Size() >= size {
			return &Reader{r: &bufioReaderAdapter{Reader: reader}, order: Order}, nil
		}
		return nil, ErrAlreadyBuffered

	// underlying is a buf so we don't need buffering
	case *BytesReader:
		return &Reader{r: reader, order: Order}, nil
	case *bytes.Reader:
		return &Reader{r: &bytesReaderAdapter{reader}, order: Order}, nil
	case *bytes.Buffer:
		return &Reader{r: &bytesBufferReaderAdapter{Buffer: reader}, order: Order}, nil
	}

	if size < 16 {
		return nil, ErrSizeTooSmall
	}

	// default use bufio
	return &Reader{
		r:     &bufioReaderAdapter{Reader: bufio.NewReaderSize(r, size), seeker: ForwardSeeker(r)},
		order: Order,
	}, nil
}

// NewReader creates a new Reader with a default buffer size.
func NewReader(r io.Reader) (*Reader, error) {
	return NewReaderSize(r, 0)
}

// WithByteOrder allows setting a custom byte order and returns
// the configured for chaining.
func (r *Reader) WithByteOrder(order binary.ByteOrder) *Reader {
	r.order = order
	return r
}

// Close closes the underlying reader if it implements io.Closer.
func (r *Reader) Close() error {
	return r.r.Close()
}

// Read implements the io.Reader interface.
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.r.Read(p)
	r.count += int64(n)
	r.setError(err)
	return n, r.err
}

// Seek moves the read pointer.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	if r.err != nil {
		return r.count, r.err
	}
	newPos, err := r.r.Seek(offset, whence)
	r.count = newPos
	r.setError(err)
	return newPos, err
}

// WriteTo implements io.WriterTo for efficient copying.
func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	if w == nil {
		r.setError(ErrWriteToNil)
		return 0, r.err
	}

	n, err := r.r.WriteTo(w)
	r.count += n
	r.setError(err)
	return n, r.err
}

func (r *Reader) Size() int    { return r.r.Size() }
func (r *Reader) Count() int64 { return r.count }
func (r *Reader) Err() error   { return r.err }
func (r *Reader) IsEOF() bool  { return r.err == io.EOF }

// setError records the first non-nil error.
func (r *Reader) setError(err error) {
	if r.err == nil && err != nil {
		r.err = err
	}
}

// Result returns the total bytes read and the final error state.
func (r *Reader) Result() (int64, error) {
	return r.count, r.err
}

// ReadTo reads data from this reader into an io.ReaderFrom.
func (r *Reader) ReadTo(w io.ReaderFrom) {
	if r.err != nil {
		return
	}
	if w == nil {
		r.setError(ErrReadToNil)
		return
	}
	n, err := w.ReadFrom(r.r)
	r.count += n
	r.setError(err)
}

// readFull is an internal helper to read an exact number of bytes.
func (r *Reader) readFull(n int) []byte {
	if r.err != nil {
		return nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		if err == io.EOF {
			// To provide a more specific error for callers;
			// a partial read is different from a clean end-of-stream.
			r.err = io.ErrUnexpectedEOF
		} else {
			r.err = err
		}
		return nil
	}
	return buf
}

// ReadBytes reads n bytes and returns a new byte slice.
func (r *Reader) ReadBytes(n int) []byte {
	if n <= 0 {
		return nil
	}
	return r.readFull(n)
}

func (r *Reader) ReadBytesTo(dest []byte) {
	if r.err != nil {
		return
	}
	if len(dest) == 0 {
		return
	}
	if _, err := io.ReadFull(r, dest); err != nil {
		r.err = err
	}
}

// Align discard bytes until offset algin with give n.
func (r *Reader) Align(n int) {
	if n > 1 {
		Discard(r, Roundup(r.count, int64(n))-r.count)
	}
}

// --- Primitive Read Operations ---

func (r *Reader) ReadBool(dest *bool) {
	if r.err != nil {
		return
	}
	b, err := r.r.ReadByte()
	if err == nil {
		r.count++
		*dest = b != 0
	} else {
		r.err = err
	}
}

func (r *Reader) ReadByte() (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	b, err := r.r.ReadByte()
	if err == nil {
		r.count++
	} else {
		r.err = err
	}
	return b, err
}

func (r *Reader) ReadUint8(dest *uint8) {
	if r.err != nil {
		return
	}
	b, err := r.r.ReadByte()
	if err == nil {
		r.count++
		*dest = b
	} else {
		r.err = err
	}
}

func (r *Reader) ReadUint16(dest *uint16) {
	buf := r.readFull(2)
	if r.err == nil {
		*dest = r.order.Uint16(buf)
	}
}

func (r *Reader) ReadUint32(dest *uint32) {
	buf := r.readFull(4)
	if r.err == nil {
		*dest = r.order.Uint32(buf)
	}
}

func (r *Reader) ReadUint64(dest *uint64) {
	buf := r.readFull(8)
	if r.err == nil {
		*dest = r.order.Uint64(buf)
	}
}

func (r *Reader) ReadInt8(dest *int8) {
	if r.err != nil {
		return
	}
	b, err := r.r.ReadByte()
	if err == nil {
		r.count++
		*dest = int8(b)
	} else {
		r.err = err
	}
}

func (r *Reader) ReadInt16(dest *int16) {
	buf := r.readFull(2)
	if r.err == nil {
		*dest = int16(r.order.Uint16(buf))
	}
}

func (r *Reader) ReadInt32(dest *int32) {
	buf := r.readFull(4)
	if r.err == nil {
		*dest = int32(r.order.Uint32(buf))
	}
}

func (r *Reader) ReadInt64(dest *int64) {
	buf := r.readFull(8)
	if r.err == nil {
		*dest = int64(r.order.Uint64(buf))
	}
}
