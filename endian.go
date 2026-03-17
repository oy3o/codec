package codec

import "encoding/binary"

var (
	BE = binary.BigEndian
	LE = binary.LittleEndian
)

// Endianness defines an implicit interface for compile-time byte order injection.
type Endianness interface {
	ByteOrder() binary.ByteOrder
}

// Endian is a Zero-Sized Type (ZST) satisfying codec.Endianness constraint.
type LittleEndian struct{}

// ByteOrder is resolved statically, no performance penalty.
func (LittleEndian) ByteOrder() binary.ByteOrder { return binary.LittleEndian }

type BigEndian struct{}

func (BigEndian) ByteOrder() binary.ByteOrder { return binary.BigEndian }
