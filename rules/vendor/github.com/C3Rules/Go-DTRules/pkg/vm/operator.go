package vm

import (
	"strings"
)

var (
	opXdef  = operator{name: "xdef"}
	opEPush = operator{name: "epush"}
	opExec  = operator{name: "exec"}

	opIsNull = unaryOp("isNull", asValue, func(x Value) Value { return boolean(x == null{}) })
	opPop    = operator{"pop", func(s State) error { _, err := s.Data().Pop(1); return err }}
	opEPop   = operator{"epop", func(s State) error { _, err := s.Entity().Pop(1); return err }}
	opReq    = binaryOp("req", asValue, func(x, y Value) Value { return boolean(EqualValues(x, y)) })

	opDup = operator{"dup", func(s State) error {
		var err error
		x := tryPop(&err, s, 1)[0]
		tryPush(&err, s, x, x)
		return err
	}}

	opSwap = operator{"swap", func(s State) error {
		var err error
		x := tryPop(&err, s, 2)
		tryPush(&err, s, x[1], x[0])
		return err
	}}
)

func init() {
	opXdef.fn = func(s State) error {
		var err error
		name := tryPopAs(&err, s, 1, as[Name])[0]
		value := tryPop(&err, s, 1)[0]
		field := tryResolve(&err, s, name)
		if err == nil {
			err = field.Store(value)
		}
		return err
	}

	opEPush.fn = func(s State) error {
		var err error
		entity := tryPopAs(&err, s, 1, as[Entity])[0]
		tryPushEntity(&err, s, entity)
		return err
	}

	opExec.fn = func(s State) error {
		var err error
		value := tryPop(&err, s, 1)[0]
		if err != nil {
			return err
		}

		// If the value is a literal, convert it to an executable
		value, err = AsExecutable(value)
		if err != nil {
			return err
		}

		return Execute(s, value)
	}
}

func resolveOperator(name string) (Value, bool) {
	if v, ok := resolveBooleanOperator(name); ok {
		return v, true
	} else if v, ok := resolveNumericOperator(name); ok {
		return v, true
	} else if v, ok := resolveArrayOperator(name); ok {
		return v, true
	} else if v, ok := resolveStringOperator(name); ok {
		return v, true
	} else if v, ok := resolveConvertOperator(name); ok {
		return v, true
	} else if v, ok := resolveControlOperator(name); ok {
		return v, true
	} else if v, ok := resolveDateTimeOperator(name); ok {
		return v, true
	}

	switch strings.ToLower(name) {
	case "xdef":
		return opXdef, true
	case "entitypush":
		return opEPush, true
	case "entitypop":
		return opEPop, true
	case "execute":
		return opExec, true
	case "isnull":
		return opIsNull, true
	case "pop":
		return opPop, true
	case "dup":
		return opDup, true
	case "swap", "exch":
		return opSwap, true
	case "null":
		return Null, true
	case "req":
		return opReq, true
	}
	return nil, false
}

type operator struct {
	name string
	fn   func(State) error
}

func (o operator) Type() Type            { return NameType }
func (o operator) String() string        { return o.name }
func (o operator) Execute(s State) error { return o.fn(s) }

func unaryOp[V any](name string, as func(Value) (V, error), fn func(x V) Value) operator {
	return op(name, 1, as, func(v []V) Value { return fn(v[0]) })
}

func binaryOp[V any](name string, as func(Value) (V, error), fn func(x, y V) Value) operator {
	return op(name, 2, as, func(v []V) Value { return fn(v[0], v[1]) })
}

func unaryOpErr[V any](name string, as func(Value) (V, error), fn func(x V) (Value, error)) operator {
	return opErr(name, 1, as, func(v []V) (Value, error) { return fn(v[0]) })
}

func binaryOpErr[V any](name string, as func(Value) (V, error), fn func(x, y V) (Value, error)) operator {
	return opErr(name, 2, as, func(v []V) (Value, error) { return fn(v[0], v[1]) })
}

func binaryOp2[X, Y any](name string, asX func(Value) (X, error), asY func(Value) (Y, error), fn func(x X, y Y) Value) operator {
	return operator{name, func(s State) error {
		var err error
		v := tryPop(&err, s, 2)
		x := tryAs(&err, v[0], asX)
		y := tryAs(&err, v[1], asY)
		tryPushResult2(&err, s, fn, x, y)
		return err
	}}
}

func op[V any](name string, n int, as func(Value) (V, error), fn func(v []V) Value) operator {
	return operator{name, func(s State) error {
		var err error
		v := tryPopAs(&err, s, n, as)
		tryPushResult1(&err, s, fn, v)
		return err
	}}
}

func opErr[V any](name string, n int, as func(Value) (V, error), fn func(v []V) (Value, error)) operator {
	return operator{name, func(s State) error {
		var err error
		v := tryPopAs(&err, s, n, as)
		tryPushResult(&err, s, func() (Value, error) { return fn(v) })
		return err
	}}
}
