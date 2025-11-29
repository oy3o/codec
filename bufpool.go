package codec

import (
	"bytes"
	"sync"
)

// bytesBufPool reuses buffers for decoding variable-length data.
// This reduces GC pressure by avoiding frequent allocations. We pool *bytes.Buffer
// because they are easily reset and resized.
var bytesBufPool = sync.Pool{
	New: func() any {
		// A 4KB default is chosen to avoid re-allocations for common packet sizes.
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

const CHUNK_SIZE = 32 * 1024

// We need a buffer to read chunks into. 32KB is a common default size used by io.Copy.
var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, CHUNK_SIZE)
		return &b
	},
}
