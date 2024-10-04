package json

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var typeU = reflect.TypeOf(new(Unmarshaller)).Elem()

type Unmarshaller interface {
	UnmarshalJSONV2(*Decoder) error
}

func Unmarshal(b []byte, value any) error {
	return NewDecoder(b).Decode(value)
}

func (d *Decoder) decodeFast(v any) (bool, error) {
	switch v := v.(type) {
	case *[]byte:
		if v == nil {
			return true, fmt.Errorf("cannot unmarshal into nil %T", v)
		}
		var err error
		*v, err = d.decodeHex()
		return true, err

	case *[32]byte:
		if v == nil {
			return true, fmt.Errorf("cannot unmarshal into nil %T", v)
		}
		b, err := d.decodeHex()
		if err != nil {
			return true, err
		}
		*v = [32]byte(b)
		return true, nil

	case Unmarshaller:
		return true, v.UnmarshalJSONV2(d)

	case *json.RawMessage:
		if v == nil {
			return true, fmt.Errorf("cannot unmarshal into nil %T", v)
		}
		*v = d.capture()
		return true, d.err

	case json.Unmarshaler:
		b := d.capture()
		if d.err != nil {
			return true, d.err
		}
		return true, v.UnmarshalJSON(b)
	}
	return false, nil
}

func (d *Decoder) decode(v any) (bool, error) {
	if rv, ok := v.(reflect.Value); ok {
		// If the value is a reflect.Value, avoid calling
		// [reflect.Value.Interface] if possible, since it allocates
		if ok, err := d.decodeReflect(rv); ok {
			return true, err
		}
		if ok, err := d.decodeFast(rv.Interface()); ok {
			return true, err
		}

	} else {
		// If the value is not a reflect.Value, avoid reflection if possible
		if ok, err := d.decodeFast(v); ok {
			return true, err
		}
		if ok, err := d.decodeReflect(reflect.ValueOf(v)); ok {
			return true, err
		}
	}
	return false, nil
}

func (d *Decoder) decodeReflect(v reflect.Value) (bool, error) {
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true, errors.New("cannot unmarshal nil")
		}
		v = v.Elem()
	}

	if !v.CanSet() {
		if v.Kind() != reflect.Pointer {
			return true, errors.New("cannot unmarshal into non pointer value")
		}
		if v.IsNil() {
			return true, errors.New("cannot unmarshal nil")
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}

	if v.Type().Implements(typeU) {
		return true, v.Interface().(Unmarshaller).UnmarshalJSONV2(d)
	}

	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Type().Implements(typeU) {
		return true, v.Interface().(Unmarshaller).UnmarshalJSONV2(d)
	}

	var isSlice bool
	switch v.Kind() {
	case reflect.Slice:
		isSlice = true
	case reflect.Array:
		// Ok
	default:
		return false, nil
	}

	if v.Type().Elem().Kind() != reflect.Uint8 {
		return false, nil
	}

	b, err := d.decodeHex()
	if err != nil {
		return true, err
	}

	if isSlice {
		v.SetBytes(b)
		return true, nil
	}

	if len(b) != v.Len() {
		return true, fmt.Errorf("wrong length: want %v, got %v", v.Len(), len(b))
	}
	reflect.Copy(v.Slice(0, v.Len()), reflect.ValueOf(b))
	return false, nil
}

func (d *Decoder) decodeHex() ([]byte, error) {
	b := d.capture()
	if d.err != nil {
		return nil, d.err
	}
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(s)
}
