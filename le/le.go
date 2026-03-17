package le

import (
	"encoding/binary"
	"io"

	"github.com/oy3o/codec"
)

var (
	le           = binary.LittleEndian
	ByteOrder    = le
	AppendUint16 = le.AppendUint16
	AppendUint32 = le.AppendUint32
	AppendUint64 = le.AppendUint64
	GoString     = le.GoString
	PutUint16    = le.PutUint16
	PutUint32    = le.PutUint32
	PutUint64    = le.PutUint64
	String       = le.String
	Uint16       = le.Uint16
	Uint32       = le.Uint32
	Uint64       = le.Uint64
)

type Fixed[Payload any] struct {
	codec.Fixed[Payload, codec.LittleEndian]
}

// NewWriterSize creates a new Writer with a specified buffer size.
// It returns an error to prevent double-buffering, a common source of bugs.
func NewWriterSize(w io.Writer, size int) (*codec.Writer, error) {
	return codec.NewWriterSize[codec.LittleEndian](w, size)
}

// NewWriter creates a new Writer with a default buffer size.
func NewWriter(w io.Writer) (*codec.Writer, error) {
	return codec.NewWriterSize[codec.LittleEndian](w, 0)
}

// NewReaderSize creates a new Reader with a specified buffer size.
func NewReaderSize(r io.Reader, size int) (*codec.Reader, error) {
	return codec.NewReaderSize[codec.LittleEndian](r, size)
}

// NewReader creates a new Reader with a default buffer size.
func NewReader(r io.Reader) (*codec.Reader, error) {
	return codec.NewReaderSize[codec.LittleEndian](r, 0)
}
