package json

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

var typeM = reflect.TypeOf(new(Marshaller)).Elem()

type Marshaller interface {
	MarshalJSONV2(*Encoder) error
}

func Marshal(value any) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := NewEncoder(buf)
	err := enc.Encode(value)
	return buf.Bytes(), err
}

func (e *Encoder) encode(value any) error {
	if rv, ok := value.(reflect.Value); ok {
		// If the value is a reflect.Value, avoid calling
		// [reflect.Value.Interface] if possible, since it allocates
		if ok, err := e.encodeReflect(rv); ok {
			return err
		}
		if ok, err := e.encodeFast(rv.Interface()); ok {
			return err
		}

	} else {
		// If the value is not a reflect.Value, avoid reflection if possible
		if ok, err := e.encodeFast(value); ok {
			return err
		}
		if ok, err := e.encodeReflect(reflect.ValueOf(value)); ok {
			return err
		}
	}

	return e.encodeStd(value)
}

func (e *Encoder) encodeFast(value any) (bool, error) {
	switch value := value.(type) {
	case nil:
		return true, e.encodeStd(nil)

	case []byte:
		return true, e.encodeStd(hex.EncodeToString(value))
	case *[]byte:
		if value == nil {
			return true, e.encodeStd(nil)
		}
		return true, e.encodeStd(hex.EncodeToString(*value))

	case [32]byte:
		return true, e.encodeStd(hex.EncodeToString(value[:]))
	case *[32]byte:
		if value == nil {
			return true, e.encodeStd(nil)
		}
		return true, e.encodeStd(hex.EncodeToString((*value)[:]))

	case json.RawMessage:
		_, err := e.out.Write(value)
		return true, err

	case *json.RawMessage:
		if value == nil {
			return true, e.encodeStd(nil)
		}
		_, err := e.out.Write(*value)
		return true, err

	case Marshaller:
		return true, e.encodeMarshaller(value)

	case json.Marshaler:
		return true, e.encodeStd(value)
	}
	return false, nil
}

func (e *Encoder) encodeReflect(v reflect.Value) (bool, error) {
	if !v.IsValid() {
		return true, e.encodeStd(nil)
	}

	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true, e.encodeStd(nil)
		}
		v = v.Elem()
	}

again:
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return true, e.encodeStd(nil)
	}

	if v.Type().Implements(typeM) {
		return true, e.encodeMarshaller(v.Interface().(Marshaller))
	}

	switch v.Kind() {
	case reflect.Pointer:
		v = v.Elem()
		goto again

	case reflect.Slice:
		if v.IsNil() {
			return true, e.encodeStd(nil)
		}

	case reflect.Array:
		// Make sure the value is addressable
		v = addr(v).Elem()

	default:
		return false, nil
	}
	if v.Type().Elem().Kind() != reflect.Uint8 {
		return false, nil
	}
	if v.Type().Name() != "" {
		return false, nil
	}

	return true, e.encodeStd(hex.EncodeToString(v.Bytes()))
}

func (e *Encoder) encodeStd(value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = e.out.Write(b)
	return err
}

func (e *Encoder) encodeMarshaller(value Marshaller) error {
	f := *e
	f.state = []emitState{emitAtInit}
	err := value.MarshalJSONV2(&f)
	if err != nil {
		return err
	}
	err = f.Done()
	if err != nil {
		return fmt.Errorf("marshaller wrote incomplete value: %w", err)
	}
	return nil
}

func addr(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return v.Addr()
	}
	u := reflect.New(v.Type())
	u.Elem().Set(v)
	return u
}
