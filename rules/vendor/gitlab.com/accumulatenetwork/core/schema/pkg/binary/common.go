// Package binary provides encoding and decoding support for a simple,
// deterministic binary encoding scheme.
//
// Fundamentally, this scheme is simple: it encodes an object as a set of fields
// where each field is tagged with an identifier. A field identifier must be an
// integer between 1 and 31, inclusive.
//
// A field is encoded as it's identifier, encoded as a byte, followed by its
// data. An object is simply a sequence of fields. Fields can be omitted but
// they cannot be encoded out of order. Attempting to encode or decode field 2
// followed by field 1 results in an error. A field may be repeated arbitrarily.
// If an object would otherwise be empty - if all fields are omitted -
// [EmptyObject] is written.
//
// This package supports directly encoding a few types:
//
//   - A boolean is written as a byte, either 1 (true) or 0 (false).
//   - An integer is written as a varint, signed or unsigned.
//   - A float is written as a big-endian IEEE 754 64-bit float.
//   - A byte slice or string is written as the length as an unsigned varint followed by data.
//   - A hash (a fixed-length 32-byte value) is written directly without modification.
//
// Nested objects are encoded then written as a byte slice - the length as an
// unsigned varint followed by data. Values that cannot be represented as one of
// the above types must implement their own encoding and are written as a byte
// slice.
//
// # Minimizing memory use
//
// Use [Pool] and type-specific methods such as [Encoder.EncodeInt] to minimize
// memory use during encoding and decoding.
//
// Type-specific methods require the [io.Writer] or [io.Reader] to implement
// additional methods to limit memory use.
//
//   - Zero-allocation encoding and decoding of booleans and integers requires [io.ByteWriter] and [io.ByteReader].
//   - Zero-allocation encoding and decoding of strings and byte slices requires [io.ByteWriter] and [io.ByteReader].
//   - Zero-allocation encoding of strings also requires [io.StringWriter].
//   - When encoding if the writer does not also implement [LenWriter], an internal writer will be used and other methods of the writer will be masked.
//   - A [Pool] is used to manage buffers used for encoding nested objects. Reusing a [Pool] across multiple encoder invocations will reduce allocations.
package binary

import (
	"bytes"
	"encoding"
	"errors"
	"io"
	"reflect"

	"gitlab.com/accumulatenetwork/core/schema/pkg/traverse"
)

// EmptyObject is written when an object would otherwise be empty.
const EmptyObject = 0x80

// MaxFieldID is the maximum field identifier.
const MaxFieldID = 31

// ErrInvalidFieldNumber is returned when an invalid field number is encountered.
var ErrInvalidFieldNumber = errors.New("field number is invalid")

// ErrFieldsOutOfOrder is returned when an out of order field is encountered.
var ErrFieldsOutOfOrder = errors.New("fields are out of order")

var errOverflow = errors.New("binary: varint overflows a 64-bit integer")

var typeM1 = reflect.TypeFor[encoding.BinaryMarshaler]()
var typeM2 = reflect.TypeFor[Marshaller]()
var typeU1 = reflect.TypeFor[encoding.BinaryUnmarshaler]()
var typeU1f = reflect.TypeFor[unmarshallerV1From]()
var typeU2 = reflect.TypeFor[Unmarshaller]()

// Marshaller is the interface implemented by types that can marshal a binary
// description of themselves.
type Marshaller interface {
	MarshalBinaryV2(*Encoder) error
}

type unmarshallerV1From = interface {
	UnmarshalBinaryFrom(rd io.Reader) error
}

// Unmarshaller is the interface implemented by types that can unmarshal a
// binary description of themselves.
type Unmarshaller interface {
	UnmarshalBinaryV2(*Decoder) error
}

// Option is an option for constructing an [Encoder] or [Decoder].
type Option func(o *options)

// WithContext sets the context of an [Encoder]. Not applicable to [Decoder].
// This can be used to preserve the context across nested invocations.
func WithContext(ctx *traverse.Context[reflect.Value]) Option {
	return func(o *options) {
		o.context = ctx
	}
}

// WithBufferPool sets the buffer pool of an [Encoder]. This can be used to
// reduce allocations. Not applicable to [Decoder].
func WithBufferPool(pool *Pool[*bytes.Buffer]) Option {
	return func(o *options) {
		o.bufferPool = pool
	}
}

// IgnoreFieldOrder disables enforcement of field ordering.
func IgnoreFieldOrder() Option {
	return func(o *options) {
		o.ignoreFieldOrder = true
	}
}

// LeaveTrailing instructs the [Decoder] to leave trailing data untouched. Not
// applicable to [Encoder].
func LeaveTrailing() Option {
	return func(o *options) {
		o.leaveTrailing = true
	}
}

type options struct {
	context          *traverse.Context[reflect.Value]
	bufferPool       *Pool[*bytes.Buffer]
	ignoreFieldOrder bool
	leaveTrailing    bool
}

func applyOptions(o *options, fn []Option) {
	for _, fn := range fn {
		fn(o)
	}
}

type common struct {
	options
	lastField       []uint
	previousOptions []options
}

func (c *common) Reset(opts ...Option) {
	applyOptions(&c.options, opts)
	c.lastField = c.lastField[:0]
	c.previousOptions = c.previousOptions[:0]
}

// WithOptions applies the given options. WithOptions will panic unless it is
// called after http://asdf.com StartObject and before Field. Changes are reverted by EndObject.
func (c *common) WithOptions(opts ...Option) {
	i := len(c.lastField) - 1
	if i < 0 {
		panic("cannot be called before StartObject")
	}
	if c.lastField[i] != 0 {
		panic("cannot be called after Field")
	}

	applyOptions(&c.options, opts)
}

func (c *common) pushScope() {
	c.lastField = append(c.lastField, 0)
	c.previousOptions = append(c.previousOptions, c.options)
}

func (c *common) popScope() {
	i := len(c.lastField) - 1
	c.lastField = c.lastField[:i]

	i = len(c.previousOptions) - 1
	c.options = c.previousOptions[i]
	c.previousOptions[i] = options{}
	c.previousOptions = c.previousOptions[:i]
}
