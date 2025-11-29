package codec

import "errors"

var (
	// ErrNilIO indicates that NewReader/NewWriter was called with an nil interface
	ErrNilIO = errors.New("codec: NewReader/NewWriter called with a nil io.Reader/io.Writer")

	// ErrSizeTooSmall indicates a size conflict with bufio
	ErrSizeTooSmall = errors.New("codec: NewReaderSize with a size smaller than 16 conflict with bufio")

	// ErrAlreadyBuffered indicates that NewReader/NewWriter was called with an already-buffered
	// reader/writer, which would lead to unpredictable behavior and performance issues.
	ErrAlreadyBuffered = errors.New("codec: reader or writer is already buffered")

	// ErrWriteToNil indicates a WriteTo operation was attempted on a nil io.Writer.
	ErrWriteToNil = errors.New("codec: WriteTo called with a nil io.Writer")

	// ErrReadToNil indicates a ReadTo operation was attempted on a nil io.ReaderFrom.
	ErrReadToNil = errors.New("codec: ReadTo called with a nil io.ReaderFrom")

	// ErrInvalidSeek indicates a seek was attempted to invalid position.
	ErrInvalidSeek = errors.New("codec: seek to a invalid position")

	// ErrUnsupportedNegativeSeek indicates a backward seek was attempted on a forward-only seeker.
	ErrUnsupportedNegativeSeek = errors.New("codec: unsupported negative offset for forward-only seeker")

	// ErrInvalidWhence indicates that an invalid 'whence' parameter was provided to a Seek operation.
	ErrInvalidWhence = errors.New("codec: unsupported whence for forward-only seeker")

	// ErrInvalidWrite indicates that an io.Writer returned an invalid (negative) count from Write.
	ErrInvalidWrite = errors.New("codec: writer returned invalid count from Write")

	// ErrInvalidRead indicates that an io.Reader returned an invalid (negative or outbound) count from Read.
	ErrInvalidRead = errors.New("codec: reader returned invalid count from Read")

	// ErrDiscardNegative indicates a Discard operation was attempted with a negative byte count.
	ErrDiscardNegative = errors.New("codec: cannot discard negative number of bytes")

	// ErrTrailingData is returned by UnmarshalBinaryGeneric when non-zero bytes are found
	// after the expected end of the data structure, indicating a potential parsing error or malformed data.
	ErrTrailingData = errors.New("codec: non-zero trailing data found after decoding")

	// ErrTruncatedData indicates that a read operation could not complete because the
	// underlying data source (e.g., buffer, stream) ended before all expected bytes were read.
	ErrTruncatedData = errors.New("codec: truncated data")
)
