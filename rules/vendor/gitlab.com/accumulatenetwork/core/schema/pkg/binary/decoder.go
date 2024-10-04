package binary

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"unsafe"
)

// Decoder decodes a binary-encoded object.
type Decoder struct {
	scanner
}

// NewDecoder creates a new decoder for the reader.
func NewDecoder(rd io.Reader, opts ...Option) *Decoder {
	d := new(Decoder)
	d.Reset(rd, opts...)
	return d
}

// Reset is equivalent to [NewDecoder]. Using Reset for repeatedly decoding
// objects reduces memory pressure.
func (d *Decoder) Reset(rd io.Reader, opts ...Option) {
	d.common.Reset(opts...)
	d.in.rd = asByteScanner(rd)
	d.state = append(d.state[:0], scanAtInit)
}

// InField returns true if the decoder just read a field ID.
//
// This is a hack and somewhat exposes the internal state of the decoder, but I
// can't think of a cleaner way of handling arrays (given that changing the
// encoding isn't an option).
func (d *Decoder) InField() bool {
	return d.current() == scanInField
}

// StartObject marks the start of an object.
func (d *Decoder) StartObject() error {
	_, err := d.scan(scanStartObject)
	return err
}

// EndObject marks the end of an object.
func (d *Decoder) EndObject() error {
	_, err := d.scan(scanEndObject)
	return err
}

// Field read a field identifier. If there are no remaining fields, Field
// returns [io.EOF].
func (d *Decoder) Field() (uint, error) {
	v, err := d.scan(scanField)
	return uint(v), err
}

// NoField indicates that the caller will decode a value that is *not* prefixed
// with a field identifier.
func (d *Decoder) NoField() error {
	_, err := d.scan(scanNoField)
	return err
}

// Decode decodes a value. The value must be a pointer to one of the supported
// types or a type that implements [Unmarshaller] or
// [encoding.BinaryUnmarshaler].
//
// Decode accepts [reflect.Value] and decodes into the underlying value.
func (d *Decoder) Decode(v any) error {
	if rv, ok := v.(reflect.Value); ok {
		// If the value is a reflect.Value, avoid calling
		// [reflect.Value.Interface] if possible, since it allocates
		if ok, err := d.decodeValueReflect(rv); ok {
			return err
		}
		if ok, err := d.decodeValueFast(rv.Interface()); ok {
			return err
		}

	} else {
		// If the value is not a reflect.Value, avoid reflection if possible
		if ok, err := d.decodeValueFast(v); ok {
			return err
		}
		if ok, err := d.decodeValueReflect(reflect.ValueOf(v)); ok {
			return err
		}
	}

	return fmt.Errorf("unable to binary marshal %T", v)
}

func (d *Decoder) decodeValueFast(v any) (bool, error) {
	var err error
	switch v := v.(type) {
	case nil:
		return true, errors.New("cannot decode into nil")

	case *bool:
		*v, err = d.DecodeBool()
		return true, err

	case *int64:
		*v, err = d.DecodeInt()
		return true, err

	case *uint64:
		*v, err = d.DecodeUint()
		return true, err

	case *float64:
		*v, err = d.DecodeFloat()
		return true, err

	case *[32]byte:
		*v, err = d.DecodeHash()
		return true, err

	case *[]byte:
		*v, err = d.DecodeBytes()
		return true, err

	case *string:
		*v, err = d.DecodeString()
		return true, err

	case Unmarshaller:
		return true, v.UnmarshalBinaryV2(d)

	case unmarshallerV1From:
		return true, d.DecodeValueFrom(v)

	case encoding.BinaryUnmarshaler:
		return true, d.DecodeValue(v)
	}
	return false, nil
}

func (d *Decoder) decodeValueReflect(v reflect.Value) (bool, error) {
	if !v.IsValid() {
		return true, errors.New("cannot unmarshal nil")
	}

	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true, errors.New("cannot unmarshal nil")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Pointer {
		if !v.CanAddr() {
			return true, errors.New("cannot unmarshal a non-pointer value")
		}
		v = v.Addr()
	}
	if v.IsNil() {
		return true, errors.New("cannot unmarshal nil")
	}

	typ := v.Type()
	elem := typ.Elem()
	ptrPtr := elem.Kind() == reflect.Pointer
	if ptrPtr && v.Elem().IsNil() {
		// The value is a pointer-pointer and the inner value is nil, so
		// initialize it
		v.Elem().Set(reflect.New(elem.Elem()))
	}

	// Check if the type or the underlying type implements either marshalling
	// interface. Don't try unmarshalling the inner value unless it's also a
	// pointer type.
	switch {
	case typ.Implements(typeU2):
		return true, d.DecodeValueV2(v.Interface().(Unmarshaller))
	case typ.Implements(typeU1f):
		return true, d.DecodeValueFrom(v.Interface().(unmarshallerV1From))
	case typ.Implements(typeU1):
		return true, d.DecodeValue(v.Interface().(encoding.BinaryUnmarshaler))

	case ptrPtr && elem.Implements(typeU2):
		return true, d.DecodeValueV2(v.Elem().Interface().(Unmarshaller))
	case ptrPtr && elem.Implements(typeU1f):
		return true, d.DecodeValueFrom(v.Elem().Interface().(unmarshallerV1From))
	case ptrPtr && elem.Implements(typeU1):
		return true, d.DecodeValue(v.Elem().Interface().(encoding.BinaryUnmarshaler))
	}

	switch elem.Kind() {
	case reflect.Bool:
		u, err := d.DecodeBool()
		v.Elem().SetBool(u)
		return true, err

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		u, err := d.DecodeInt()
		v.Elem().SetInt(u)
		return true, err

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := d.DecodeUint()
		v.Elem().SetUint(u)
		return true, err

	case reflect.Float32, reflect.Float64:
		u, err := d.DecodeFloat()
		v.Elem().SetFloat(u)
		return true, err

	case reflect.String:
		u, err := d.DecodeString()
		v.Elem().SetString(u)
		return true, err
	}

	return false, nil
}

// DecodeBool reads a byte and interprets it as a boolean. DecodeBool returns an
// error if the value is not 0 or 1.
func (d *Decoder) DecodeBool() (bool, error) {
	_, err := d.scan(scanFixedWidth)
	if err != nil {
		return false, err
	}

	b, err := d.in.ReadByte()
	if err != nil {
		return false, err
	}

	switch b {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("cannot convert %d to a bool", b)
	}
}

// DecodeInt reads a signed varint.
func (d *Decoder) DecodeInt() (int64, error) {
	ux, err := d.scan(scanVarWidth)
	if err != nil {
		return 0, err
	}

	// Copied from [binary.PutVarint]
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return x, nil
}

// DecodeUint reads an unsigned varint.
func (d *Decoder) DecodeUint() (uint64, error) {
	return d.scan(scanVarWidth)
}

// DecodeFloat reads a big-endian, 64-bit IEEE 754 floating point number.
func (d *Decoder) DecodeFloat() (float64, error) {
	_, err := d.scan(scanFixedWidth)
	if err != nil {
		return 0, err
	}

	var b [8]byte
	_, err = io.ReadFull(&d.in, b[:])
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b[:])), nil
}

// DecodeHash reads a hash.
func (d *Decoder) DecodeHash() ([32]byte, error) {
	_, err := d.scan(scanFixedWidth)
	if err != nil {
		return [32]byte{}, err
	}

	var v [32]byte
	_, err = io.ReadFull(&d.in, v[:])
	return v, err
}

// DecodeBytes reads a length-prefixed byte slice.
func (d *Decoder) DecodeBytes() ([]byte, error) {
	n, err := d.scan(scanVarWidth)
	if err != nil {
		return nil, err
	}

	v := make([]byte, n)
	_, err = io.ReadFull(&d.in, v)
	return v, err
}

// DecodeString reads a length-prefixed string.
func (d *Decoder) DecodeString() (string, error) {
	b, err := d.DecodeBytes()
	if len(b) == 0 || err != nil {
		return "", err
	}

	// Use unsafe to avoid allocating. The only way this is not safe is if the
	// reader keeps the buffer and mutates it after the fact, which should not
	// happen.
	return unsafe.String(&b[0], len(b)), nil
}

// DecodeValue reads a length-prefixed byte slice and calls
// [encoding.BinaryUnmarshaler.UnmarshalBinary].
func (d *Decoder) DecodeValue(v encoding.BinaryUnmarshaler) error {
	b, err := d.DecodeBytes()
	if err != nil {
		return err
	}
	return v.UnmarshalBinary(b)
}

// DecodeValue reads a length-prefixed value unmarshalled with
// UnmarshalBinaryFrom.
func (d *Decoder) DecodeValueFrom(v unmarshallerV1From) error {
	// Treat the value as an object. This will read the length prefix and set up
	// the limit reader for UnmarshalBinaryFrom.
	err := d.StartObject()
	if err != nil {
		return err
	}

	err = v.UnmarshalBinaryFrom(&d.in)
	if err != nil {
		return err
	}

	return d.EndObject()
}

// DecodeValue calls [Unmarshaller.UnmarshalBinaryV2].
func (d *Decoder) DecodeValueV2(v Unmarshaller) error {
	return v.UnmarshalBinaryV2(d)
}
