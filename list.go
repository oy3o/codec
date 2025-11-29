package codec

import (
	"io"
	"reflect"
)

type List interface {
	Codec
	Len() int
	Codecs() []Codec
}

// listOptions defines the configuration for encoding and decoding a list of codecs.
type listOptions struct {
	// Alignment specifies the byte boundary to which each item (except the last) should be padded.
	// A value of 0 or 1 means no alignment. Common values are 4 or 8.
	Alignment int
}

// list is a generic, high-performance codec for handling slices of any type
// that implements the Codec interface. It supports alignment padding and
// streaming reads and writes.
type list[T Codec] struct {
	Items   []T
	options *listOptions
}

// Statically ensure that List implements Codec.
var _ List = (*list[Codec])(nil)

func (l *list[T]) Codecs() []Codec {
	codecs := make([]Codec, l.Len())
	for i, codec := range l.Items {
		codecs[i] = codec
	}
	return codecs
}

type (
	List0[T Codec] struct{ list[T] }
	List4[T Codec] struct{ list[T] }
	List8[T Codec] struct{ list[T] }
)

// NewList creates a new List codec with the given items and options.
func NewList[T Codec](items []T, options *listOptions) *list[T] {
	if options == nil {
		options = &listOptions{Alignment: 0}
	}
	return &list[T]{
		Items:   items,
		options: options,
	}
}

// no Align type List
func NewList0[T Codec](items []T) *List0[T] {
	return &List0[T]{list[T]{Items: items, options: &listOptions{Alignment: 0}}}
}

// 4 Align type List
func NewList4[T Codec](items []T) *List4[T] {
	return &List4[T]{list[T]{Items: items, options: &listOptions{Alignment: 4}}}
}

// 8 Align type List
func NewList8[T Codec](items []T) *List8[T] {
	return &List8[T]{list[T]{Items: items, options: &listOptions{Alignment: 8}}}
}

func (l *list[T]) Len() int {
	return len(l.Items)
}

// Size calculates the total binary size of the list, including alignment padding.
func (l *list[T]) Size() int {
	if len(l.Items) == 0 {
		return 0
	}

	totalSize := 0
	lastIndex := len(l.Items) - 1

	for i, item := range l.Items {
		itemSize := item.Size()
		totalSize += itemSize
		// Add padding for all items except the last one.
		if i < lastIndex && l.options.Alignment > 1 {
			padding := Roundup(itemSize, l.options.Alignment) - itemSize
			totalSize += padding
		}
	}
	return totalSize
}

// WriteTo efficiently writes the entire list to a writer, handling alignment.
func (l *list[T]) WriteTo(writer io.Writer) (int64, error) {
	if len(l.Items) == 0 {
		return 0, nil
	}

	w, _ := NewWriter(writer)
	lastIndex := len(l.Items) - 1

	for i, item := range l.Items {
		w.WriteFrom(item)

		if i < lastIndex && l.options.Alignment > 1 {
			w.Align(l.options.Alignment)
		}
	}
	return w.Result()
}

// ReadFrom reads and decodes items into the list from a reader.
// The read behavior is determined by the capacity of the `l.Items` slice:
// - If cap(l.Items) > 0, it reads exactly that many items.
// - If cap(l.Items) == 0, it reads items until the reader returns io.EOF.
func (l *list[T]) ReadFrom(reader io.Reader) (int64, error) {
	var n int64
	count := cap(l.Items)
	readEOF := count == 0

	for i := 0; readEOF || i < count; i++ {
		// Create a new instance of the concrete type T for decoding into.
		var item T

		elemType := reflect.TypeOf(item)
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		newItem := reflect.New(elemType).Interface().(T)

		// Try to read the next item.
		read, err := newItem.ReadFrom(reader)
		n += read

		if err != nil {
			if readEOF && (err == io.EOF || read == 0) {
				// Clean EOF when reading indefinitely: this is the success termination condition.
				break
			}
			// Any other error (including UnexpectedEOF or EOF on a fixed-size read) is a failure.
			return n, err
		}

		l.Items = append(l.Items, newItem)

		// Determine if padding should be consumed.
		isLastItem := !readEOF && (i == count-1)

		if !isLastItem && l.options.Alignment > 1 {
			padding := Roundup(read, int64(l.options.Alignment)) - read
			if padding > 0 {
				skipped, err := Discard(reader, padding)
				n += skipped

				if err != nil {
					if readEOF && (err == io.EOF || read == 0) {
						// Clean EOF when reading indefinitely: this is the success termination condition.
						break
					}
					// Any other error (including UnexpectedEOF or EOF on a fixed-size read) is a failure.
					return n, err
				}
			}
		}
	}

	return n, nil
}

// --- Boilerplate implementations ---

func (l *list[T]) MarshalBinary() ([]byte, error) {
	return MarshalBinaryGeneric(l)
}

func (l *list[T]) UnmarshalBinary(data []byte) error {
	return UnmarshalBinaryGeneric(l, data)
}

func (l *list[T]) MarshalTo(buf []byte) (int, error) {
	return MarshalToGeneric(l, buf)
}
