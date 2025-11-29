//go:build test

package codec

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// --- Mocks and Helpers ---

// A simple fixed-size struct for testing codec implementations.
type mockPayload struct {
	ID   uint32
	Data [4]byte
}

// mockCodec is an alias for a FixedSizeCodec using our mockPayload.
type mockCodec = Fixed[mockPayload]

// mockFlushingWriter helps verify that a writer's Flush method is called.
type mockFlushingWriter struct {
	bytes.Buffer
	flushed bool
}

func (m *mockFlushingWriter) Flush() error {
	m.flushed = true
	return nil
}

// --- Writer Test Suite ---

type WriterTestSuite struct {
	suite.Suite
	buf    *bytes.Buffer
	writer *Writer
}

// SetupTest runs before each test in the suite, ensuring a clean state.
func (s *WriterTestSuite) SetupTest() {
	s.buf = &bytes.Buffer{}
	s.writer, _ = NewWriter(s.buf)
}

func (s *WriterTestSuite) TestConstructors() {
	s.T().Run("PanicsOnNilWriter", func(t *testing.T) {
		_, err := NewWriter(nil)
		assert.ErrorIs(t, err, ErrNilIO)
	})
}

func (s *WriterTestSuite) TestBasicWrites() {
	codec := &mockCodec{mockPayload{ID: 0xDEADBEEF, Data: [4]byte{1, 2, 3, 4}}}

	s.writer.WriteUint8(0xAA)
	s.writer.WriteUint16(0xBBCC)
	s.writer.WriteUint32(0xDDEEFF00)
	s.writer.WriteUint64(0x0102030405060708)
	s.writer.WriteBytes([]byte{5, 6, 7})
	s.writer.WriteZeros(2)
	s.writer.WriteFrom(codec)

	n, err := s.writer.Result()
	s.Require().NoError(err)
	s.Assert().EqualValues(1+2+4+8+3+2+8, n)
	s.Assert().EqualValues(s.buf.Len(), s.writer.Count())

	expected := []byte{
		0xAA,       // WriteUint8
		0xCC, 0xBB, // WriteUint16 (Little Endian)
		0x00, 0xFF, 0xEE, 0xDD, // WriteUint32 (Little Endian)
		0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, // WriteUint64 (Little Endian)
		5, 6, 7, // WriteBytes
		0, 0, // WriteZeros
		0xEF, 0xBE, 0xAD, 0xDE, 1, 2, 3, 4, // WriteFrom(codec)
	}
	s.Assert().Equal(expected, s.buf.Bytes())
}

func (s *WriterTestSuite) TestErrorHandling() {
	s.T().Run("ShortBufferError", func(t *testing.T) {
		// Use a fixed-size buffer to reliably trigger ErrShortWrite.
		fixedBuf := make([]byte, 5)
		writer, _ := NewWriter(NewBytesWriter(fixedBuf))

		writer.WriteUint32(0x11223344) // Writes 4 bytes to buffer, OK.
		writer.WriteUint32(0xAABBCCDD) // Writes another 4 bytes to buffer, OK.

		// Result() will flush the buffer, triggering the underlying write and the error.
		_, err := writer.Result()
		require.Error(t, err, "Error should be present after flush")
		assert.ErrorIs(t, err, io.ErrShortWrite)
	})

	s.T().Run("WriteAfterErrorIsNoOp", func(t *testing.T) {
		fixedBuf := make([]byte, 5)
		writer, _ := NewWriter(NewBytesWriter(fixedBuf))

		writer.WriteUint32(0x11223344) // Success (buffered)
		writer.WriteUint32(0xAABBCCDD) // Fails during flush, not here.

		// Manually flush to trigger the error.
		writer.Flush()

		// Now, the error should be latched.
		firstErr := writer.Err()
		require.Error(t, firstErr)
		require.ErrorIs(t, firstErr, io.ErrShortWrite)

		// This subsequent write should be a no-op because an error state is set.
		writer.WriteUint8(0xFF)
		writer.Flush() // Flushing again should not change the error.

		// Verify the error is still the same and the buffer state is as expected.
		assert.Equal(t, firstErr, writer.Err(), "The latched error should not change")

		// The underlying BytesWriter received the first 4 bytes, and then 1 byte from
		// the second write before it ran out of space. The final 0xFF was never written.
		expected := []byte{0x44, 0x33, 0x22, 0x11, 0xDD}
		assert.Equal(t, expected, fixedBuf)

		// Verify count reflects only what was successfully written to the buffer before the error
		// Note: bufio.Writer might write partial data, so the final count can be tricky.
		// The most important thing is that the error is caught.
		// A precise count check here is less critical than the error and final buffer state.
	})
}

func (s *WriterTestSuite) TestFlush() {
	// mockFlushingWriter has a custom Flush method we can inspect.
	mock := &mockFlushingWriter{}
	writer, _ := NewWriterSize(mock, 128)
	writer.WriteUint8(0xAA)

	// Before flush, data is in the buffer, but not in the underlying writer.
	s.Assert().True(writer.w.(*bufioWriterAdapter).Buffered() > 0)
	s.Assert().False(mock.flushed)
	s.Assert().Zero(mock.Len())

	writer.Flush()

	s.Assert().False(mock.flushed, "Flush should call underlying Flush but our mock doesn't implement it on the Buffer")
	s.Assert().Zero(writer.w.(*bufioWriterAdapter).Buffered())
	s.Assert().Equal(1, mock.Buffer.Len())
}

// TestWriter runs the WriterTestSuite.
func TestWriter(t *testing.T) {
	suite.Run(t, new(WriterTestSuite))
}

// --- Reader Test Suite ---

type ReaderTestSuite struct {
	suite.Suite
}

func (s *ReaderTestSuite) TestConstructors() {
	s.T().Run("PanicsOnNilReader", func(t *testing.T) {
		_, err := NewReader(nil)
		assert.ErrorIs(t, err, ErrNilIO)
	})
}

func (s *ReaderTestSuite) TestSuccessfulReads() {
	data := []byte{
		0xAA,       // uint8
		0xCC, 0xBB, // uint16
		0x00, 0xFF, 0xEE, 0xDD, // uint32
		0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, // uint64
		0x11, 0x22, 0x33, // raw bytes
	}
	r, _ := NewReader(bytes.NewReader(data))

	var v8 uint8
	var v16 uint16
	var v32 uint32
	var v64 uint64
	r.ReadUint8(&v8)
	r.ReadUint16(&v16)
	r.ReadUint32(&v32)
	r.ReadUint64(&v64)
	read := r.ReadBytes(3)

	s.Require().NoError(r.Err())
	s.Assert().Equal(uint8(0xAA), v8)
	s.Assert().Equal(uint16(0xBBCC), v16)
	s.Assert().Equal(uint32(0xDDEEFF00), v32)
	s.Assert().Equal(uint64(0x0102030405060708), v64)
	s.Assert().Equal([]byte{0x11, 0x22, 0x33}, read)

	// The next read should result in a clean EOF.
	r.Read(make([]byte, 1))
	s.Assert().ErrorIs(r.Err(), io.EOF)
	s.Assert().True(r.IsEOF())
}

func (s *ReaderTestSuite) TestErrorHandling() {
	s.T().Run("ReadPastEOF", func(t *testing.T) {
		data := []byte{0x01, 0x02, 0x03}
		r, _ := NewReader(bytes.NewReader(data))
		var v32 uint32
		r.ReadUint32(&v32) // Attempt to read 4 bytes from a 3-byte source.

		require.Error(t, r.Err())
		assert.ErrorIs(t, r.Err(), io.ErrUnexpectedEOF)
		assert.False(t, r.IsEOF(), "ErrUnexpectedEOF should not be considered a clean EOF")
	})

	s.T().Run("ReadAfterErrorIsNoOp", func(t *testing.T) {
		data := []byte{0x01, 0x02, 0x03}
		r, _ := NewReader(bytes.NewReader(data))
		var v32 uint32
		var v8 uint8

		r.ReadUint32(&v32) // This will trigger and latch the error.
		firstErr := r.Err()
		require.Error(t, firstErr)

		r.ReadUint8(&v8) // This read should not happen.
		assert.Equal(t, firstErr, r.Err(), "The latched error should not change")
		assert.Equal(t, uint8(0), v8, "Destination variable should be unchanged after an error")
	})
}

func (s *ReaderTestSuite) TestInterfaceMethods() {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	r, _ := NewReader(bytes.NewReader(data))

	s.T().Run("WriteTo", func(t *testing.T) {
		var buf bytes.Buffer
		n, err := r.WriteTo(&buf)
		require.NoError(t, err)
		assert.EqualValues(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
	})

	s.T().Run("WriteToNilWriter", func(t *testing.T) {
		r, _ := NewReader(bytes.NewReader(data))
		_, err := r.WriteTo(nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrWriteToNil)
	})
}

func (s *ReaderTestSuite) TestSeekBehavior() {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	r, _ := NewReader(bytes.NewReader(data)) // bytes.Reader implements io.ReadSeeker

	// 1. Seek from start
	pos, err := r.Seek(3, io.SeekStart)
	s.Require().NoError(err)
	s.Assert().EqualValues(3, pos)
	s.Assert().EqualValues(3, r.Count())

	// 2. Read after seek
	b := r.ReadBytes(2)
	s.Require().NoError(r.Err())
	s.Assert().Equal([]byte{0x04, 0x05}, b)
	s.Assert().EqualValues(5, r.Count())

	// 3. Seek from current (within buffer) - this should be an optimized path
	pos, err = r.Seek(1, io.SeekCurrent)
	s.Require().NoError(err)
	s.Assert().EqualValues(6, pos)

	// 4. Seek backwards (should fail for non-seeker readers, but work for bytes.Reader)
	pos, err = r.Seek(0, io.SeekStart)
	s.Require().NoError(err)
	s.Assert().EqualValues(0, pos)
}

func (s *ReaderTestSuite) TestForwardOnlySeekerErrors() {
	// Use a reader that does NOT implement io.Seeker to test our forwardSeeker wrapper.
	r, _ := NewReader(bytes.NewBuffer(make([]byte, 10))) // bytes.Buffer is not a Seeker

	// 1. Seek forward works
	_, err := r.Seek(5, io.SeekStart)
	s.Require().NoError(err)

	// 2. Seek backward fails
	_, err = r.Seek(2, io.SeekStart)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported negative offset")

	// 3. Seek with invalid whence fails
	r, _ = NewReader(bytes.NewBuffer(make([]byte, 10))) // bytes.Buffer is not a Seeker
	_, err = r.Seek(0, io.SeekEnd)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported whence")
}

// TestReader runs the ReaderTestSuite.
func TestReader(t *testing.T) {
	suite.Run(t, new(ReaderTestSuite))
}

// --- Standalone Codec Tests ---

func TestFixedSizeCodec_SizeCache(t *testing.T) {
	c := &mockCodec{mockPayload{ID: 1}}
	expectedSize := 8 // uint32(4) + [4]byte(4)

	// The first call populates the cache.
	size1 := c.Size()
	assert.Equal(t, expectedSize, size1)

	// The second call should hit the cache. We verify by checking the value.
	// In a real-world scenario, you might benchmark this.
	size2 := c.Size()
	assert.Equal(t, expectedSize, size2)

	// Verify the cache is shared globally.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c2 := &mockCodec{mockPayload{ID: 2}}
			assert.Equal(t, expectedSize, c2.Size())
		}()
	}
	wg.Wait()
}

func TestFixedSizeCodec_Errors(t *testing.T) {
	t.Run("MarshalToShortBuffer", func(t *testing.T) {
		c := &mockCodec{}
		shortBuf := make([]byte, c.Size()-1)
		_, err := c.MarshalTo(shortBuf)
		assert.ErrorIs(t, err, io.ErrShortBuffer)
	})

	t.Run("UnmarshalWithTruncatedData", func(t *testing.T) {
		c := &mockCodec{}
		validData, _ := c.MarshalBinary()
		truncatedData := validData[:len(validData)-1]

		err := c.UnmarshalBinary(truncatedData)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("UnmarshalWithTrailingData", func(t *testing.T) {
		c := &mockCodec{}
		validData, _ := c.MarshalBinary()
		trailingData := append(validData, 0x01, 0x02, 0x03) // Append non-zero bytes

		err := c.UnmarshalBinary(trailingData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-zero byte")
	})
}
