package codec

import (
	"encoding/binary"
	"io"
	"reflect"

	"github.com/puzpuzpuz/xsync/v4"
)

// sizeCache avoids the high performance cost of reflection in `binary.Size`
// on every call. Using a global sync.Map makes it concurrent-safe.
var sizeCache = xsync.NewMap[reflect.Type, int]()

// Fixed provides a generic `Codec` implementation for any struct `Body`
// composed of fixed-size fields, eliminating boilerplate for simple data structures.
//
// Constraint: The `Body` type MUST NOT contain variable-size fields like slices,
// maps, or strings, as this will cause `binary.Size` to fail.
type Fixed[Payload any] struct {
	Payload Payload
}

// Statically assert that FixedSizeCodec implements Codec.
var _ Codec = (*Fixed[struct{}])(nil)

// Size returns the fixed size of the struct in bytes.
// The result is cached to avoid reflection overhead on subsequent calls.
func (c *Fixed[Payload]) Size() int {
	bodyType := reflect.TypeOf((*Payload)(nil)).Elem()

	// Attempt to load from the concurrent-safe cache first for performance.
	if size, ok := sizeCache.Load(bodyType); ok {
		return size
	}

	// If not cached, perform the expensive reflection-based calculation.
	size := binary.Size(&c.Payload)

	// Store the result for subsequent calls.
	sizeCache.Store(bodyType, size)
	return size
}

// MarshalBinary implements the standard `encoding.BinaryMarshaler` interface.
// Note: This method allocates a new byte slice. For performance-critical paths,
// use `MarshalTo` or `WriteTo` instead.
func (c *Fixed[Payload]) MarshalBinary() ([]byte, error) {
	buf := make([]byte, c.Size())
	if _, err := binary.Encode(buf, Order, &c.Payload); err != nil {
		return nil, io.ErrShortWrite // binary.Encode only returns unexported buffer too small error, it means fewer bytes were written than expected
	}
	return buf, nil
}

// UnmarshalBinary implements the standard `encoding.BinaryUnmarshaler` interface.
// It calls `CheckTrailingNotZeros` to prevent bugs from truncated or oversized payloads.
func (c *Fixed[Payload]) UnmarshalBinary(data []byte) error {
	n, err := binary.Decode(data, Order, &c.Payload)
	if err != nil {
		return ErrTruncatedData // binary.Decode always returns unexported buffer too small error, it means the data is truncated
	}
	if len(data) > n {
		if err := CheckBufferNotZeros(data[n:]); err != nil {
			return err // Ensure no trailing zeros in the buffer
		}
	}
	return nil
}

// ReadFrom implements `io.ReaderFrom` for efficient, allocation-free reading
// directly from a stream into the struct.
func (c *Fixed[Payload]) ReadFrom(r io.Reader) (int64, error) {
	err := binary.Read(r, Order, &c.Payload)
	if err != nil {
		return 0, err
	}
	return int64(c.Size()), nil
}

// WriteTo implements `io.WriterTo` for efficient, allocation-free writing
// directly to a stream (e.g., a network connection or file).
func (c *Fixed[Payload]) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, Order, &c.Payload)
	if err != nil {
		return 0, err
	}
	return int64(c.Size()), nil
}

// MarshalTo marshals the struct into the provided slice `p`.
// This is the most performant marshalling option as it avoids memory allocation.
func (c *Fixed[Payload]) MarshalTo(p []byte) (int, error) {
	n, err := binary.Encode(p, Order, &c.Payload)
	if err != nil {
		return n, io.ErrShortWrite // binary.Encode only returns unexported buffer too small error, it means fewer bytes were written than expected
	}
	return n, nil
}
