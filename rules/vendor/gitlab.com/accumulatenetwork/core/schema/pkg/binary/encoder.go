package binary

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"

	"gitlab.com/accumulatenetwork/core/schema/pkg/traverse"
)

// Encoder binary-encodes an object.
type Encoder struct {
	emitter
}

// NewEncoder creates a new encoder for the writer.
func NewEncoder(w io.Writer, opts ...Option) *Encoder {
	e := new(Encoder)
	e.Reset(w, opts...)
	return e
}

// Reset is equivalent to [NewEncoder]. Using Reset for repeatedly encoding
// objects reduces memory pressure.
func (e *Encoder) Reset(w io.Writer, opts ...Option) {
	e.common.Reset(opts...)
	e.outs = append(e.outs[:0], asLenWriter(w))
	e.state = append(e.state[:0], emitAtInit)

	if e.bufferPool == nil {
		e.bufferPool = NewPointerPool[bytes.Buffer]()
	}
}

// InField returns true if the decoder just read a field ID.
//
// This is a hack and somewhat exposes the internal state of the decoder, but I
// can't think of a cleaner way of handling arrays (given that changing the
// encoding isn't an option).
func (e *Encoder) InField() bool { return e.current() == emitInField }

// Context returns the [traverse.Context] associated with the encoder, creating
// a new one if needed. Context is provided for the caller, it is not used by
// the encoder.
func (e *Encoder) Context() *traverse.Context[reflect.Value] {
	if e.context == nil {
		e.context = new(traverse.Context[reflect.Value])
	}
	return e.context
}

// StartObject marks the start of an object. StartObject does not write
// anything. If the object is nested, StartObject configures the encoder to
// write the nested object to a buffer retrieved from a [Pool].
func (e *Encoder) StartObject() error {
	err := e.emit(emitStartObject)
	if err != nil {
		return err
	}

	// Is it a nested object?
	if e.current() == emitAtInit {
		return nil
	}

	return nil
}

// EndObject marks the end of an object. If the object would otherwise be empty
// - if no fields were written - EndObject writes [EmptyObject]. If the object
// was nested, EndObject writes the length of the object then copies contents of
// the intermediate buffer to the output.
func (e *Encoder) EndObject() error {
	return e.emit(emitEndObject)
}

// Field writes a field identifier, verifying that it is a valid field number
// and is not out of sequence.
func (e *Encoder) Field(n uint) error {
	if len(e.lastField) == 0 {
		return errors.New("attempted to write a field number outside of an object")
	}
	i := len(e.lastField) - 1
	if !e.ignoreFieldOrder && n < e.lastField[i] {
		return ErrFieldsOutOfOrder
	}
	if n < 1 || n > MaxFieldID {
		return ErrInvalidFieldNumber
	}
	e.lastField[i] = n
	return e.encode(emitField, uint64(n))
}

func (e *Encoder) NoField() error {
	if len(e.lastField) == 0 {
		return errors.New("attempted to write a field number outside of an object")
	}
	return e.emit(emitField)
}

// RepeatLastField number repeats the last call to [Encoder.Field]. If
// [Encoder.Encode] has not been called since the last call to [Encoder.Field],
// RepeatLastField is a no-op.
func (e *Encoder) RepeatLastField() error {
	// Check the current state
	switch s := e.current(); s {
	case emitInField:
		// Field number was just written
		return nil

	case emitInObject:
		n := e.lastField[len(e.lastField)-1]
		if n == 0 {
			return fmt.Errorf("cannot repeat last field: no field has been written")
		}

		// Repeat the last field number
		return e.encode(emitField, n)

	default:
		// Not in an object
		return emitField.error(s)
	}
}

// EncodeField calls [Encoder.Field] and [Encoder.Encode].
func (e *Encoder) EncodeField(n uint, v any) error {
	err := e.Field(n)
	if err != nil {
		return err
	}
	return e.Encode(v)
}

// Encode encodes a value. The value must be one of the supported types or a
// type that implements [Marshaller] or [encoding.BinaryMarshaler].
//
// Encode accepts [reflect.Value] and encodes the underlying value.
func (e *Encoder) Encode(v any) error {
	return e.encode(emitValue, v)
}

// EncodeBool writes a single byte, 0 (false) or 1 (true).
func (e *Encoder) EncodeBool(v bool) error { return e.emitBool(emitValue, v) }

// EncodeInt writes a signed varint.
func (e *Encoder) EncodeInt(v int64) error { return e.emitInt(emitValue, v) }

// EncodeUint writes an unsigned varint.
func (e *Encoder) EncodeUint(v uint64) error { return e.emitUint(emitValue, v) }

// EncodeFloat writes a big-endian, 64-bit IEEE 754 floating point number.
func (e *Encoder) EncodeFloat(v float64) error { return e.emitFloat(emitValue, v) }

// EncodeBytes writes the length (unsigned varint) followed by the data.
func (e *Encoder) EncodeBytes(v []byte) error { return e.emitBytes(emitValue, v) }

// EncodeString writes the length (unsigned varint) followed by the data.
func (e *Encoder) EncodeString(v string) error { return e.emitString(emitValue, v) }

// EncodeHash writes the hash as bytes.
func (e *Encoder) EncodeHash(v [32]byte) error { return e.emitHash(emitValue, v) }

// EncodeValue calls [encoding.BinaryMarshaler.MarshalBinary] and writes it as a
// length-prefixed byte slice.
func (e *Encoder) EncodeValue(v encoding.BinaryMarshaler) error { return e.emitM1(emitValue, v) }

// EncodeValueV2 calls [Marshaller.MarshalBinaryV2].
func (e *Encoder) EncodeValueV2(v Marshaller) error { return e.emitM2(v) }

// Done returns an error if the object was not completed.
func (e *Encoder) Done() error {
	if len(e.state) > 0 {
		return errors.New("not done")
	}
	return nil
}

func (e *Encoder) encode(a emitAction, v any) error {
	if rv, ok := v.(reflect.Value); ok {
		// If the value is a reflect.Value, avoid calling
		// [reflect.Value.Interface] if possible, since it allocates
		if ok, err := e.encodeValueReflect(a, rv); ok {
			return err
		}
		if ok, err := e.encodeValueFast(a, rv.Interface()); ok {
			return err
		}

	} else {
		// If the value is not a reflect.Value, avoid reflection if possible
		if ok, err := e.encodeValueFast(a, v); ok {
			return err
		}
		if ok, err := e.encodeValueReflect(a, reflect.ValueOf(v)); ok {
			return err
		}
	}

	return fmt.Errorf("unable to binary marshal %T", v)
}

func (e *Encoder) encodeValueFast(a emitAction, v any) (bool, error) {
	switch v := v.(type) {
	case nil:
		return true, errors.New("cannot marshal nil")

	case bool:
		return true, e.emitBool(a, v)

	case int64:
		return true, e.emitInt(a, v)

	case uint64:
		return true, e.emitUint(a, v)

	case float64:
		return true, e.emitFloat(a, v)

	case [32]byte:
		return true, e.emitHash(a, v)

	case *[32]byte:
		return true, e.emitHash(a, *v)

	case []byte:
		return true, e.emitBytes(a, v)

	case string:
		return true, e.emitString(a, v)

	case Marshaller:
		return true, e.emitM2(v)

	case encoding.BinaryMarshaler:
		return true, e.emitM1(a, v)
	}
	return false, nil
}

func (e *Encoder) encodeValueReflect(a emitAction, v reflect.Value) (bool, error) {
	if !v.IsValid() {
		return true, errors.New("cannot marshal nil")
	}

	// If the type is a pointer, check its underlying type. If the type is not a
	// pointer, check its pointer type.
	typ := v.Type()
	var elem, ptr reflect.Type
	if typ.Kind() == reflect.Pointer {
		elem = typ.Elem()
	} else {
		ptr = reflect.PointerTo(typ)
	}

	// Check if the type or it's underlying/pointer type implements either
	// marshalling interface
	switch {
	case typ.Implements(typeM1):
		return true, e.emitM1(a, v.Interface().(encoding.BinaryMarshaler))
	case typ.Implements(typeM2):
		return true, e.emitM2(v.Interface().(Marshaller))

	case elem != nil && elem.Implements(typeM1):
		return true, e.emitM1(a, v.Elem().Interface().(encoding.BinaryMarshaler))
	case elem != nil && elem.Implements(typeM2):
		return true, e.emitM2(v.Elem().Interface().(Marshaller))

	case ptr != nil && ptr.Implements(typeM1):
		return true, e.emitM1(a, addr(v).Interface().(encoding.BinaryMarshaler))
	case ptr != nil && ptr.Implements(typeM2):
		return true, e.emitM2(addr(v).Interface().(Marshaller))
	}

	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return true, errors.New("cannot marshal nil")
		}
		return e.encodeValueReflect(a, v.Elem())

	case reflect.Bool:
		return e.encodeValueFast(a, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.encodeValueFast(a, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return e.encodeValueFast(a, v.Uint())
	case reflect.Float32, reflect.Float64:
		return e.encodeValueFast(a, v.Float())
	case reflect.String:
		return e.encodeValueFast(a, v.String())
	}

	return false, nil
}

func addr(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return v.Addr()
	}
	u := reflect.New(v.Type())
	u.Elem().Set(v)
	return u
}

func (e *Encoder) emitBool(a emitAction, v bool) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeBool(e.out(), v)
}

func (e *Encoder) emitInt(a emitAction, v int64) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeInt(e.out(), v)
}

func (e *Encoder) emitUint(a emitAction, v uint64) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeUint(e.out(), v)
}

func (e *Encoder) emitFloat(a emitAction, v float64) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeFloat(e.out(), v)
}

func (e *Encoder) emitBytes(a emitAction, v []byte) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeBytes(e.out(), v)
}

func (e *Encoder) emitHash(a emitAction, v [32]byte) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	// TODO(ethan): Does this allocate? Can we eliminate that?
	_, err = e.out().Write(v[:])
	return err
}

func (e *Encoder) emitString(a emitAction, v string) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeString(e.out(), v)
}

func (e *Encoder) emitM1(a emitAction, v encoding.BinaryMarshaler) error {
	err := e.emit(a)
	if err != nil {
		return err
	}

	return writeM1(e.out(), v)
}

func (e *Encoder) emitM2(v Marshaller) error {
	return v.MarshalBinaryV2(e)
}
