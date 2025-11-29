package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/exp/constraints"
)

var (
	BE = binary.BigEndian
	LE = binary.LittleEndian
	// Order is default binary order
	Order = BE
)

const BUFFER_SIZE = 4096

var (
	empty   [BUFFER_SIZE]byte
	discard [BUFFER_SIZE]byte
)

func Ptr[T any](v T) *T { return &v } // ptr is a helper function to create a pointer to a value, making test setup cleaner.

func Discard(r io.Reader, n int64) (int64, error) {
	if n == 0 {
		return 0, nil
	}
	if n < 0 {
		return 0, ErrDiscardNegative
	}
	if n <= BUFFER_SIZE {
		skip, err := r.Read(discard[:n])
		return int64(skip), err
	}
	return io.CopyN(io.Discard, r, n)
}

// Roundup rounds n up to the nearest multiple of align.
func Roundup[T constraints.Integer](n, align T) T { return (n + (align - 1)) &^ (align - 1) }

// MAX_PADDING defines the maximum number of trailing bytes to check.
// This prevents an Out-Of-Memory error if a parsing bug leaves a large
// amount of data in the reader. Anything larger is considered a protocol error.
const MAX_PADDING = 1024 // 1KB

// CheckTrailingNotZeros verifies that any remaining bytes in a reader are all zero.
// This is critical for parsers to ensure the entire expected payload was consumed
// and no garbage data follows, which could indicate a bug or a malicious payload.
func CheckTrailingNotZeros(r io.Reader) error {
	// Fast path for a common reader type to avoid any allocations.
	if reader, ok := r.(*BytesReader); ok && reader.Available() == 0 {
		return nil
	}

	// Use a LimitedReader to enforce our heuristic limit. We read up to
	// `maxExpectedPadding + 1` bytes; if the read succeeds, we know there was
	// too much data.
	lr := &io.LimitedReader{R: r, N: MAX_PADDING + 1}

	trailingData, err := io.ReadAll(lr)
	if err != nil {
		return err
	}

	// Heuristic check: Did we read more than the allowed padding size?
	if len(trailingData) > MAX_PADDING {
		return fmt.Errorf("%w: exceeds maximum expected size of %d bytes", ErrTrailingData, MAX_PADDING)
	}

	// Check if the data we did read contains non-zero bytes.
	for i, b := range trailingData {
		if b != 0 {
			return fmt.Errorf("%w: found non-zero byte 0x%02x at offset %d", ErrTrailingData, b, i)
		}
	}

	return nil
}

// ReadVariableFieldStream is a helper function to read a variable-length field from an stream
// and stream its content directly to a destination writer. It avoids allocating memory for the
// field's data, making it suitable for large payloads like file transfers.
//
// Parameters:
//   - dst: The io.Writer where the field's data will be written. If nil, data is discarded.
//   - r: The io.Reader to read from (the packet stream).
//   - currentReadOffset: The total number of bytes read from the start of the packet so far.
//   - targetFieldOffset: The offset of the desired field from the start of the packet.
//   - targetFieldLength: The length of the desired field.
//
// Returns:
//   - int64: The total number of bytes consumed from the source reader 'r' (including padding).
//   - error: An error if reading fails or the packet is malformed.
func ReadVariableFieldStream(dst io.Writer, r io.Reader, currentReadOffset, targetFieldOffset, targetFieldLength int64) (int64, error) {
	if targetFieldLength == 0 {
		return 0, nil
	}

	var n int64 = 0

	// 1. Calculate and skip any padding before the field.
	written, err := Discard(r, targetFieldOffset-currentReadOffset)
	n += written
	if err != nil {
		return n, err
	}

	// 2. Stream the field's data directly to the destination writer.
	if dst == nil {
		dst = io.Discard
	}
	written, err = io.CopyN(dst, r, targetFieldLength)
	n += written
	if err != nil {
		// io.CopyN returns io.EOF if the stream ends, which we should wrap for clarity.
		return n, err
	}

	return n, nil
}

// ReadVariableField is a helper function to read a variable-length field from an stream.
// This function is a convenience wrapper around ReadVariableFieldStream, using an in-memory
// buffer as the destination. It is suitable for smaller payloads. For large payloads,
// ReadVariableFieldStream should be used directly to avoid memory allocation.
//
// Parameters and return values are the same as the original function.
func ReadVariableField(r io.Reader, currentReadOffset, targetFieldOffset, targetFieldLength int64) ([]byte, int64, error) {
	// 1. If there's nothing to read, return immediately.
	if targetFieldLength == 0 || targetFieldOffset < currentReadOffset {
		return nil, 0, nil
	}

	// 2. Use a BytesWriter as the in-memory writer.
	buf := NewBytesWriter(make([]byte, int(targetFieldLength)))

	// 3. Delegate all the complex I/O logic to the streaming version.
	bytesConsumed, err := ReadVariableFieldStream(buf, r, currentReadOffset, targetFieldOffset, targetFieldLength)
	if err != nil {
		// If streaming failed, we might have partial data in the buffer,
		// but we should return a nil slice along with the error.
		return nil, bytesConsumed, err
	}

	// 5. On success, return the buffer's content and the total bytes consumed from the source reader.
	return buf.Bytes(), bytesConsumed, nil
}

// ReadStringUntilNullOrEOF reads bytes from an io.Reader until a null character (0)
// or an io.EOF is encountered.
//
// To ensure efficient reading, this function first checks if the provided reader
// implements the io.ByteReader interface. If so, it uses the efficient ReadByte() method.
func ReadUntilNullOrEOF(r io.Reader) ([]byte, int64, error) {
	var str bytes.Buffer

	// Check if the reader has an efficient ReadByte method.
	if br, ok := r.(io.ByteReader); ok {
		for {
			b, err := br.ReadByte()
			if err != nil {
				if err == io.EOF {
					break // Cleanly handle end of file.
				}
				return nil, int64(str.Len()), err // Return on any other error.
			}

			if b == 0 {
				break // Null terminator found.
			}
			str.WriteByte(b)
		}
	} else {
		// Fallback for readers without ReadByte.
		// This is less efficient but guarantees we don't buffer and
		// consume extra bytes from the reader.
		p := make([]byte, 1)
		for {
			n, err := r.Read(p)

			// Always process the byte first if it was read (n > 0).
			// This correctly handles the case where Read returns both n=1 and err=io.EOF.
			if n > 0 {
				if p[0] == 0 {
					break // Null terminator found.
				}
				str.WriteByte(p[0])
			}

			// After processing the byte, check for the error condition.
			if err != nil {
				if err == io.EOF {
					break // Reached the end.
				}
				return nil, int64(str.Len()), err // A real error occurred.
			}
		}
	}

	return str.Bytes(), int64(str.Len()), nil
}

// ReadUTF16StringUntilNull reads a null-terminated UTF-16 string from an io.Reader.
// It detects endianness from the BOM. If no BOM is present, it defaults to Little Endian.
// It correctly handles surrogate pairs for characters outside the Basic Multilingual Plane.
//
// It returns the decoded UTF-8 string, the number of bytes read, and any error.
func ReadUTF16StringUntilNull(r io.Reader) (string, int64, error) {
	// We read 2 bytes at a time.
	p := make([]byte, 2)
	var words []uint16
	var order binary.ByteOrder
	var bytesRead int64

	// 1. Determine Endianness by reading the first two bytes for a BOM.
	n, err := io.ReadFull(r, p)
	bytesRead += int64(n)
	if err != nil {
		// If we reach EOF on the first read, it's an empty string.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return "", bytesRead, nil
		}
		return "", bytesRead, err
	}

	switch {
	case p[0] == 0xFE && p[1] == 0xFF:
		order = binary.BigEndian
	case p[0] == 0xFF && p[1] == 0xFE:
		order = binary.LittleEndian
	default:
		// No BOM found, default to Little Endian and process the bytes we already read.
		order = binary.LittleEndian
		word := order.Uint16(p)
		if word == 0 { // Null terminator was the first thing.
			return "", bytesRead, nil
		}
		words = append(words, word)
	}

	// 2. Read 2-byte words until a null word (0x0000) or EOF is found.
	for {
		n, err := io.ReadFull(r, p)
		bytesRead += int64(n)

		if n == 2 {
			word := order.Uint16(p)
			if word == 0 {
				break // Null terminator found.
			}
			words = append(words, word)
		}

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break // End of stream is a valid way to terminate.
			}
			return "", bytesRead, err // A real error occurred.
		}
	}

	// 3. Decode the UTF-16 words into a UTF-8 string.
	// This is a simplified decode loop. For full correctness including
	// surrogate pairs, a more robust library function would be better,
	// but this demonstrates the principle.
	var str []rune
	for i := 0; i < len(words); i++ {
		w1 := words[i]
		// High surrogate
		if w1 >= 0xD800 && w1 <= 0xDBFF {
			if i+1 < len(words) {
				w2 := words[i+1]
				// Low surrogate
				if w2 >= 0xDC00 && w2 <= 0xDFFF {
					r := 0x10000 + (int32(w1-0xD800)<<10 | int32(w2-0xDC00))
					str = append(str, rune(r))
					i++ // Skip the next word since we consumed it
					continue
				}
			}
			// Malformed UTF-16 (unpaired high surrogate)
			str = append(str, rune(0xFFFD)) // Replacement character
			continue
		}
		// Malformed UTF-16 (unpaired low surrogate)
		if w1 >= 0xDC00 && w1 <= 0xDFFF {
			str = append(str, rune(0xFFFD))
			continue
		}
		str = append(str, rune(w1))
	}

	return string(str), bytesRead, nil
}
