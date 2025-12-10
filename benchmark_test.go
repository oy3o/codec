package codec

import (
	"encoding/binary"
	"testing"
)

type BenchmarkPayload struct {
	ID      uint32
	Val1    uint64
	Val2    uint64
	Val3    uint64
	IsAlive bool
	Padding [3]byte
}

type BenchmarkCodec = Fixed[BenchmarkPayload]

func BenchmarkFixedMarshalBinary(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.MarshalBinary()
	}
}

func BenchmarkFixedUnmarshalBinary(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	data, _ := c.MarshalBinary()
	var c2 BenchmarkCodec
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c2.UnmarshalBinary(data)
	}
}

func BenchmarkFixedMarshalTo(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	buf := make([]byte, c.Size())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.MarshalTo(buf)
	}
}

// Baseline comparison using only binary.Write/Read directly, to see overhead of the wrapper
func BenchmarkStandardBinaryWrite(b *testing.B) {
	payload := BenchmarkPayload{ID: 1, Val1: 100}
	buf := make([]byte, binary.Size(payload))
	w := NewBytesWriter(buf) // using same writer as library
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Reset()
		_ = binary.Write(w, Order, &payload)
	}
}
