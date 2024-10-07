package vm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func AsValue(v any) (Value, error) {
	switch v := v.(type) {
	case Value:
		return v, nil
	case []Value:
		return (*LiteralArray)(&v), nil
	case *[]Value:
		return (*LiteralArray)(v), nil
	case []any:
		u := make(LiteralArray, len(v))
		for i, v := range v {
			var err error
			u[i], err = AsValue(v)
			if err != nil {
				return nil, err
			}
		}
		return &u, nil
	case time.Time:
		return datetime(v), nil
	case string:
		return LiteralString(v), nil
	case bool:
		return boolean(v), nil
	case uint:
		return Number[uint]{v}, nil
	case uint8:
		return Number[uint8]{v}, nil
	case uint16:
		return Number[uint16]{v}, nil
	case uint32:
		return Number[uint32]{v}, nil
	case uint64:
		return Number[uint64]{v}, nil
	case int:
		return Number[int]{v}, nil
	case int8:
		return Number[int8]{v}, nil
	case int16:
		return Number[int16]{v}, nil
	case int32:
		return Number[int32]{v}, nil
	case int64:
		return Number[int64]{v}, nil
	case float32:
		return Number[float32]{v}, nil
	case float64:
		return Number[float64]{v}, nil
	}
	return nil, fmt.Errorf("cannot convert %T into a Value", v)
}

func AsAny(v Value) (any, error) {
	switch v.Type() {
	case NameType:
		if v, ok := v.(Name); ok {
			return v.Name(), nil
		}

	case NumberType:
		if v, ok := v.(Numeric); ok {
			return v.AsFloat()
		}

	case BooleanType:
		if v, ok := v.(Boolean); ok {
			return v.AsBool()
		}

	case StringType:
		return v.String(), nil

	case ArrayType:
		if v, ok := v.(Array); ok {
			u := make([]any, v.Len())
			for i := range u {
				var err error
				u[i], err = AsAny(v.Get(i))
				if err != nil {
					return nil, err
				}
			}
			return u, nil
		}
	}

	if v, ok := v.(interface{ AsAny() (any, error) }); ok {
		return v.AsAny()
	}

	return nil, fmt.Errorf("cannot convert %T into a Go value", v)
}

func AsArray(v Value) (Array, error) { return asZ1[Array](v, &LiteralArray{}, errNotArray) }

func AsInt(v Value) (int64, error) { return asZ2(v, 0, errNotNumeric, Numeric.AsInt) }

func AsUint(v Value) (uint64, error) { return asZ2(v, 0, errNotNumeric, Numeric.AsUint) }

func AsFloat(v Value) (float64, error) { return asZ2(v, 0, errNotNumeric, Numeric.AsFloat) }

func AsBool(v Value) (bool, error) { return asZ2(v, false, errNotBoolean, Boolean.AsBool) }

func AsDateTime(v Value) (time.Time, error) {
	return asZ2(v, time.Time{}, errNotDateTime, DateTime.AsDateTime)
}

func AsDuration(v Value) (time.Duration, error) {
	return asZ2(v, 0, errNotDuration, Duration.AsDuration)
}

func AsString(v Value) (string, error) { return v.String(), nil }

func asValue(v Value) (Value, error) {
	return v, nil
}

func as[V any](v Value) (V, error) {
	u, ok := v.(V)
	if !ok {
		return u, fmt.Errorf("%w %v", errValueIsNot, reflect.TypeOf(new(V)).Elem())
	}
	return u, nil
}

func asZ1[V any](v Value, z V, err error) (V, error) {
	switch v := v.(type) {
	case V:
		return v, nil
	case null:
		return z, nil
	default:
		return z, err
	}
}

func asZ2[V, U any](v Value, z U, err error, fn func(V) (U, error)) (U, error) {
	switch v := v.(type) {
	case V:
		return fn(v)
	case null:
		return z, nil
	default:
		return z, err
	}
}

func PopAs[V any](s State, n int, fn func(Value) (V, error)) ([]V, error) {
	v, err := s.Data().Pop(n)
	if err != nil {
		return nil, err
	}
	u := make([]V, n)
	for i, v := range v {
		u[i], err = fn(v)
		if err != nil {
			return nil, err
		}
	}
	return u, nil
}

var (
	opCvi = convertOp("toInt", AsInt, func(x int64) Value { return Number[int64]{x} })
	opCvr = convertOp("toReal", AsFloat, func(x float64) Value { return Number[float64]{x} })
	opCvb = convertOp("toBool", AsBool, func(x bool) Value { return boolean(x) })
	opCvs = convertOp("toString", AsString, func(x string) Value { return LiteralString(x) })

	opCvn = convertOp("toName", AsString, func(x string) Value {
		if strings.HasPrefix(x, "/") {
			return ExecutableName(x[1:])
		}
		return LiteralName(x)
	})

	opCve = operator{"toEntity", func(s State) error {
		v, err := s.Data().Pop(1)
		if err != nil {
			return err
		}
		if _, ok := v[0].(Entity); !ok {
			v[0] = Null
		}
		return s.Data().Push(v...)
	}}
)

func resolveConvertOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "cvi":
		return opCvi, true
	case "cvr":
		return opCvr, true
	case "cvb":
		return opCvb, true
	case "cvs":
		return opCvs, true
	case "cvn":
		return opCvn, true
	case "cve":
		return opCve, true
	}
	return nil, false
}

func convertOp[V any](name string, as func(Value) (V, error), fn func(v V) Value) Value {
	return operator{name, func(s State) error {
		var err error
		v := tryPopAs(&err, s, 1, as)[0]
		if err == nil {
			tryPush(&err, s, fn(v))
		} else {
			err = nil
			tryPush(&err, s, Null)
		}
		return err
	}}
}
