package be

import (
	"io"

	"github.com/oy3o/codec"
)

type Fixed[Payload any] struct {
	codec.Fixed[Payload, codec.BigEndian]
}

// NewWriterSize creates a new Writer with a specified buffer size.
// It returns an error to prevent double-buffering, a common source of bugs.
func NewWriterSize(w io.Writer, size int) (*codec.Writer, error) {
	return codec.NewWriterSize[codec.BigEndian](w, size)
}

// NewWriter creates a new Writer with a default buffer size.
func NewWriter(w io.Writer) (*codec.Writer, error) {
	return codec.NewWriterSize[codec.BigEndian](w, 0)
}

// NewReaderSize creates a new Reader with a specified buffer size.
func NewReaderSize(r io.Reader, size int) (*codec.Reader, error) {
	return codec.NewReaderSize[codec.BigEndian](r, size)
}

// NewReader creates a new Reader with a default buffer size.
func NewReader(r io.Reader) (*codec.Reader, error) {
	return codec.NewReaderSize[codec.BigEndian](r, 0)
}
