package codec

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

type writer interface {
	io.Writer
	io.ReaderFrom
	io.Closer
}

type WriterPro interface {
	writer
	io.ByteWriter
	io.StringWriter
	Size() int
	Flush() error
}

// Writer provides a buffered writer that simplifies writing binary data.
// It wraps bufio.Writer for efficiency and tracks the first error that occurs.
// After an error, all subsequent write operations become no-ops.
type Writer struct {
	w     WriterPro
	count int64 // total bytes written
	err   error // first error encountered. Subsequent writes become no-ops.
	depth int
	order binary.ByteOrder
}

var _ WriterPro = (*Writer)(nil)

// NewWriterSize creates a new Writer with a specified buffer size.
// It returns an error to prevent double-buffering, a common source of bugs.
func NewWriterSize(w io.Writer, size int) (*Writer, error) {
	if w == nil {
		return nil, ErrNilIO
	}

	switch bw := w.(type) {
	// Reuse the underlying buffer if it's already a compatible Writer.
	case *Writer:
		if bw.w.Size() >= size {
			return &Writer{w: bw.w, depth: bw.depth + 1, order: Order}, nil
		}

	// prevent unpredictable double-buffering.
	case *bufio.Writer:
		if bw.Size() >= size {
			return &Writer{w: &bufioWriterAdapter{bw}, depth: 1, order: Order}, nil
		}
		return nil, ErrAlreadyBuffered

	// underlying is a buf so we don't need buffering
	case *BytesWriter:
		return &Writer{w: bw, order: Order}, nil
	case *bytes.Buffer:
		return &Writer{w: &bytesBufferWriterAdapter{bw}, order: Order}, nil
	}

	// default use bufio
	return &Writer{w: &bufioWriterAdapter{bufio.NewWriterSize(w, size)}, order: Order}, nil
}

// NewWriter creates a new Writer with a default buffer size.
func NewWriter(w io.Writer) (*Writer, error) {
	return NewWriterSize(w, 0)
}

// WithByteOrder allows setting a custom byte order and returns
// the configured for chaining.
func (w *Writer) WithByteOrder(order binary.ByteOrder) *Writer {
	w.order = order
	return w
}

// Close closes the underlying writer if it implements io.Closer.
func (w *Writer) Close() error {
	return w.w.Close()
}

// Write implements the io.Writer interface.
func (w *Writer) Write(buf []byte) (int, error) {
	if buf == nil || w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(buf)
	w.count += int64(n)
	w.setError(err)
	return n, w.err
}

// Write implements the io.StringWriter interface.
func (w *Writer) WriteString(str string) (int, error) {
	if str == "" || w.err != nil {
		return 0, w.err
	}
	n, err := w.w.WriteString(str)
	w.count += int64(n)
	w.setError(err)
	return n, w.err
}

// ReadFrom implements io.ReaderFrom for efficient copying.
func (w *Writer) ReadFrom(r io.Reader) (int64, error) {
	if r == nil || w.err != nil {
		return 0, w.err
	}
	n, err := w.w.ReadFrom(r)
	w.count += n
	w.setError(err)
	return n, w.err
}

func (w *Writer) Size() int    { return w.w.Size() }
func (w *Writer) Count() int64 { return w.count }
func (w *Writer) Err() error   { return w.err }

// setError records the first non-nil error.
// This preserves the root cause of a failure chain instead of a later,
// less relevant error.
func (w *Writer) setError(err error) {
	if w.err == nil && err != nil {
		w.err = err
	}
}

// Result flushes the buffer and returns the final count and error state.
func (w *Writer) Result() (int64, error) {
	w.Flush()
	return w.count, w.err
}

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error {
	// To prevent nested writers from flushing the buffer prematurely.
	// Only the outermost writer should be responsible for the final flush.
	if w.depth > 0 || w.err != nil {
		return w.err
	}
	err := w.w.Flush()
	w.setError(err)
	return err
}

// WriteFrom reads data from an io.WriterTo.
func (w *Writer) WriteFrom(wt io.WriterTo) {
	if wt == nil || w.err != nil {
		return
	}
	n, err := wt.WriteTo(w.w)
	w.count += n
	w.setError(err)
}

// WriteBytes writes a byte slice.
func (w *Writer) WriteBytes(buf []byte) {
	if buf == nil || w.err != nil {
		return
	}
	_, _ = w.Write(buf)
}

// WriteZeros writes n zero bytes, often for padding.
// To avoid allocating a large temporary zero buffer for big padding.
func (w *Writer) WriteZeros(n int64) {
	if w.err != nil || n <= 0 {
		return
	}
	if n <= BUFFER_SIZE {
		// To avoid heap allocation for small, common padding sizes.
		w.Write(empty[:n])
	} else {
		// Fallback to the efficient io.CopyN for larger padding.
		_, err := io.CopyN(w, Zero, n)
		w.setError(err)
	}
}

// Align write zero bytes until offset algin with give n.
func (w *Writer) Align(n int) {
	if n > 1 {
		w.WriteZeros(Roundup(w.count, int64(n)) - w.count)
	}
}

// --- Primitive Write Operations ---

func (w *Writer) WriteBool(v bool) {
	if w.err != nil {
		return
	}
	var err error
	if v {
		err = w.w.WriteByte(1)
	} else {
		err = w.w.WriteByte(0)
	}
	if err == nil {
		w.count++
	} else {
		w.err = err
	}
}

func (w *Writer) WriteByte(v byte) error {
	if w.err != nil {
		return w.err
	}
	err := w.w.WriteByte(v)
	if err == nil {
		w.count++
	} else {
		w.err = err
	}
	return err
}

func (w *Writer) WriteUint8(v uint8) {
	if w.err != nil {
		return
	}
	err := w.w.WriteByte(v)
	if err == nil {
		w.count++
	} else {
		w.err = err
	}
}

func (w *Writer) WriteUint16(v uint16) {
	if w.err != nil {
		return
	}
	var buf [2]byte
	w.order.PutUint16(buf[:], v)
	_, _ = w.Write(buf[:])
}

func (w *Writer) WriteUint32(v uint32) {
	if w.err != nil {
		return
	}
	var buf [4]byte
	w.order.PutUint32(buf[:], v)
	_, _ = w.Write(buf[:])
}

func (w *Writer) WriteUint64(v uint64) {
	if w.err != nil {
		return
	}
	var buf [8]byte
	w.order.PutUint64(buf[:], v)
	_, _ = w.Write(buf[:])
}

func (w *Writer) WriteInt8(v int8) {
	if w.err != nil {
		return
	}
	err := w.w.WriteByte(uint8(v))
	if err == nil {
		w.count++
	} else {
		w.err = err
	}
}

func (w *Writer) WriteInt16(v int16) {
	if w.err != nil {
		return
	}
	var buf [2]byte
	w.order.PutUint16(buf[:], uint16(v))
	_, _ = w.Write(buf[:])
}

func (w *Writer) WriteInt32(v int32) {
	if w.err != nil {
		return
	}
	var buf [4]byte
	w.order.PutUint32(buf[:], uint32(v))
	_, _ = w.Write(buf[:])
}

func (w *Writer) WriteInt64(v int64) {
	if w.err != nil {
		return
	}
	var buf [8]byte
	w.order.PutUint64(buf[:], uint64(v))
	_, _ = w.Write(buf[:])
}
