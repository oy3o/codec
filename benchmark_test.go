package codec

import (
	"encoding/binary"
	"io"
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

// Baseline comparison using only binary.Encode diectly, to see overhead of the wrapper
func BenchmarkStandardBinaryEncode(b *testing.B) {
	payload := BenchmarkPayload{ID: 1, Val1: 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, binary.Size(payload))
		_, _ = binary.Encode(buf, Order, &payload)
	}
}

func BenchmarkStandardBinaryEncodeWithSize(b *testing.B) {
	payload := BenchmarkPayload{ID: 1, Val1: 100}
	c := BenchmarkCodec{Payload: payload}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, c.Size())
		_, _ = binary.Encode(buf, Order, &payload)
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

// Baseline comparison using only binary.Encode diectly, to see overhead of the wrapper
func BenchmarkStandardBinaryEncodeWithBuf(b *testing.B) {
	payload := BenchmarkPayload{ID: 1, Val1: 100}
	buf := make([]byte, binary.Size(payload))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = binary.Encode(buf, Order, &payload)
	}
}

func BenchmarkFixedUnmarshalBinary(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	data, _ := c.MarshalBinary()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var c2 BenchmarkCodec
		_ = c2.UnmarshalBinary(data)
	}
}

// Baseline comparison using only binary.Decode diectly, to see overhead of the wrapper
func BenchmarkStandardBinaryDecode(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	data, _ := c.MarshalBinary()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var c2 BenchmarkPayload
		_, _ = binary.Decode(data, Order, &c2)
	}
}

func BenchmarkFixedReadFrom(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	data, _ := c.MarshalBinary()
	var c2 BenchmarkCodec
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewBytesReader(data)
		_, _ = c2.ReadFrom(r)
	}
}

// Baseline comparison using only binary.Read diectly, to see overhead of the wrapper
func BenchmarkStandardBinaryRead(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	data, _ := c.MarshalBinary()
	var c2 BenchmarkCodec
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewBytesReader(data)
		_ = binary.Read(r, Order, &c2)
		_ = binary.Size(c2)
	}
}

func BenchmarkFixedWriteTo(b *testing.B) {
	c := &BenchmarkCodec{Payload: BenchmarkPayload{ID: 1, Val1: 100}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.WriteTo(io.Discard)
	}
}

// Baseline comparison using only binary.Write diectly, to see overhead of the wrapper
func BenchmarkStandardBinaryWrite(b *testing.B) {
	payload := BenchmarkPayload{ID: 1, Val1: 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = binary.Write(io.Discard, Order, &payload)
		_ = binary.Size(payload)
	}
}
