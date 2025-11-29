package codec

import (
	"encoding"
	"io"
)

// Sizer is an interface for types that can report their binary size.
// This is useful for pre-allocating buffers before encoding.
type Sizer interface {
	// Size returns the size of the type in bytes when binary encoded.
	Size() int
}

// Marshaler defines the core methods for encoding an object into a byte stream.
// It integrates standard library interfaces and provides a high-performance,
// allocation-free option.
type Marshaler interface {
	// encoding.BinaryMarshaler provides the primary encoding method.
	// It allocates and returns a new byte slice.
	encoding.BinaryMarshaler // Method: MarshalBinary() ([]byte, error)
	// io.WriterTo provides efficient, stream-based writing.
	// This avoids allocating the entire byte slice in memory at once.
	io.WriterTo // Method: WriteTo(writer io.Writer) (int64, error)

	// MarshalTo is a high-performance, zero-allocation encoding method.
	// It encodes the object into a pre-allocated buffer, returning an error
	// (e.g., io.ErrShortBuffer) if the buffer is too small.
	MarshalTo(buf []byte) (int, error)
}

// Unmarshaler defines the core methods for decoding a byte stream into an object.
type Unmarshaler interface {
	// encoding.BinaryUnmarshaler decodes data from a byte slice.
	encoding.BinaryUnmarshaler // Method: UnmarshalBinary(data []byte) error
	// io.ReaderFrom provides efficient, stream-based reading.
	io.ReaderFrom // Method: ReadFrom(r io.Reader) (int64, error)
}

// Codec aggregates all binary serialization and deserialization interfaces.
// A type implementing Codec is a complete, self-sizing binary encoder/decoder.
type Codec interface {
	Sizer
	Marshaler
	Unmarshaler
}
