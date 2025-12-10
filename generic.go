package codec

import (
	"bytes"
	"encoding"
	"fmt"
	"io"
)

// MarshalBinaryGeneric provides a generic `encoding.BinaryMarshaler` implementation.
func MarshalBinaryGeneric[T interface {
	Size() int
	io.WriterTo
}](v T) ([]byte, error) {
	expectedSize := v.Size()
	w := NewBytesWriter(make([]byte, expectedSize))
	n, err := v.WriteTo(w)
	if err != nil {
		return nil, err
	}
	if n < int64(expectedSize) {
		return nil, fmt.Errorf("%w: expected at least %d bytes, but write %d", ErrTruncatedData, expectedSize, n)
	}
	return w.Bytes(), nil
}

// UnmarshalBinaryGeneric provides a generic `UnmarshalBinary` for types implementing `io.ReaderFrom`.
// It adapts a stream-based `ReadFrom` to the slice-based `UnmarshalBinary` interface
// and adds a crucial check for unexpected trailing data.
func UnmarshalBinaryGeneric[T interface {
	io.ReaderFrom
	Size() int
}](v T, data []byte) error {
	r := NewBytesReader(data)
	n, err := v.ReadFrom(r)
	if err != nil {
		return err
	}
	expectedSize := v.Size()

	if n < int64(expectedSize) {
		// Robustness check: Ensure the buffer wasn't truncated.
		return fmt.Errorf("%w: expected at least %d bytes, but read %d", ErrTruncatedData, expectedSize, n)
	}

	// Ensure no unexpected trailing data remains.
	// This prevents parsing ambiguous or potentially malicious payloads.
	if len(data) > int(n) {
		if err := CheckBufferNotZeros(data[n:]); err != nil {
			return err
		}
	}
	return nil
}

// ReadFromGeneric provides a generic, non-streaming `io.ReaderFrom` implementation.
// WARNING: This is NOT a streaming implementation. It reads the entire `io.Reader`
// into a memory buffer before unmarshalling. It is unsuitable for very large inputs.
func ReadFromGeneric[T encoding.BinaryUnmarshaler](v T, r io.Reader) (int64, error) {
	buf := bytesBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bytesBufPool.Put(buf)

	n, err := buf.ReadFrom(r)
	if err != nil {
		return n, err
	}
	return n, v.UnmarshalBinary(buf.Bytes())
}

// WriteToGeneric provides a generic `io.WriterTo` implementation.
// It adapts a type that can marshal to a byte slice to the streaming io.Writer interface.
func WriteToGeneric[T encoding.BinaryMarshaler](v T, w io.Writer) (int64, error) {
	buf, err := v.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(buf)
	if err != nil {
		return int64(n), err
	}
	if n < len(buf) {
		return int64(n), io.ErrShortWrite
	}
	return int64(n), nil
}

// MarshalToGeneric provides a fallback implementation for the MarshalTo method.
func MarshalToGeneric[T interface {
	Size() int
	io.WriterTo
}](v T, p []byte) (int, error) {
	size := v.Size()
	if len(p) < size {
		return 0, io.ErrShortWrite
	}
	w := NewBytesWriter(p)
	n, err := v.WriteTo(w)
	if err != nil {
		return int(n), err
	}
	if n < int64(size) {
		return int(n), io.ErrShortWrite
	}
	return int(n), nil
}
