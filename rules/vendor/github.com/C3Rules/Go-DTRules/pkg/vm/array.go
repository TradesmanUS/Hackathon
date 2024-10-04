package vm

import (
	"errors"
	"slices"
	"strings"
)

type Array interface {
	Value
	Len() int
	Get(int) Value
	Append(...Value)
	Insert(int, Value)
	Remove(Value)
	RemoveAt(int)
	Contains(Value) bool
	Clear()
}

type ArrayExecutable interface {
	ArrayExecute(State) error
}

type ExecutableArray []Value
type LiteralArray []Value

var _ Array = &ExecutableArray{}
var _ Array = &LiteralArray{}

func (a *ExecutableArray) String() string             { return "{ " + arrayToString(*a) + " }" }
func (a *ExecutableArray) AsLiteral() (Value, error)  { return (*LiteralArray)(a), nil }
func (a *ExecutableArray) Len() int                   { return len(*a) }
func (a *ExecutableArray) Get(i int) Value            { return (*a)[i] }
func (a *ExecutableArray) Append(v ...Value)          { *a = append(*a, v...) }
func (a *ExecutableArray) Insert(i int, v Value)      { *a = slices.Insert(*a, i, v) }
func (a *ExecutableArray) Remove(v Value)             { *a = slices.DeleteFunc(*a, equalTo(v)) }
func (a *ExecutableArray) RemoveAt(i int)             { *a = slices.Delete(*a, i, i+1) }
func (a *ExecutableArray) Clear()                     { *a = (*a)[:0] }
func (a *ExecutableArray) Contains(v Value) bool      { return slices.ContainsFunc(*a, equalTo(v)) }
func (a *ExecutableArray) ArrayExecute(s State) error { return s.Data().Push(a) }

func (a *ExecutableArray) Execute(s State) error {
	for _, v := range *a {
		err := arrayExecute(s, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *LiteralArray) String() string               { return "[ " + arrayToString(*a) + " ]" }
func (a *LiteralArray) AsExecutable() (Value, error) { return (*ExecutableArray)(a), nil }
func (a *LiteralArray) Len() int                     { return len(*a) }
func (a *LiteralArray) Get(i int) Value              { return (*a)[i] }
func (a *LiteralArray) Append(v ...Value)            { *a = append(*a, v...) }
func (a *LiteralArray) Insert(i int, v Value)        { *a = slices.Insert(*a, i, v) }
func (a *LiteralArray) Remove(v Value)               { *a = slices.DeleteFunc(*a, equalTo(v)) }
func (a *LiteralArray) RemoveAt(i int)               { *a = slices.Delete(*a, i, i+1) }
func (a *LiteralArray) Clear()                       { *a = (*a)[:0] }
func (a *LiteralArray) Contains(v Value) bool        { return slices.ContainsFunc(*a, equalTo(v)) }

var (
	opAStart = operator{"[", PushDataFrame}
	opAStop  = operator{"]", popAsArray[*LiteralArray]}
	opXStart = _opXStart{}
	opXStop  = _opXStop{}

	opLength   = unaryOp("len", AsArray, func(x Array) Value { return Number[int]{x.Len()} })
	opGetAt    = binaryOp2("get", AsArray, AsUint, func(x Array, y uint64) Value { return x.Get(int(y)) })
	opNewArray = operator{"newArray", func(s State) error { return s.Data().Push(new(LiteralArray)) }}
	opMemberOf = binaryOp2("contains", AsArray, asValue, func(x Array, y Value) Value { return boolean(x.Contains(y)) })

	opCopyElem = unaryOp("copyArray", AsArray, func(x Array) Value {
		switch x := x.(type) {
		case *ExecutableArray:
			return copyArray(x)
		case *LiteralArray:
			return copyArray(x)
		default:
			y := make(LiteralArray, x.Len())
			for i, n := 0, x.Len(); i < n; i++ {
				y[i] = x.Get(i)
			}
			return &y
		}
	})

	opAddTo = operator{"append", func(s State) error {
		var err error
		y := tryPop(&err, s, 1)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil {
			x.Append(y)
		}
		return err
	}}

	opAddAt = operator{"insert", func(s State) error {
		var err error
		z := tryPop(&err, s, 1)[0]
		y := tryPopAs(&err, s, 1, AsInt)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil {
			x.Insert(int(y), z)
		}
		return err
	}}

	opRemove = operator{"remove", func(s State) error {
		var err error
		y := tryPop(&err, s, 1)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil {
			x.Remove(y)
		}
		tryPush(&err, s, x)
		return err
	}}

	opRemoveAt = operator{"removeAt", func(s State) error {
		var err error
		y := tryPopAs(&err, s, 1, AsInt)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil {
			x.RemoveAt(int(y))
		}
		tryPush(&err, s, x)
		return err
	}}

	opAddUnique = operator{"addUnique", func(s State) error {
		var err error
		y := tryPop(&err, s, 1)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil && !x.Contains(y) {
			x.Append(y)
		}
		return nil
	}}

	opClear = operator{"clear", func(s State) error {
		var err error
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err == nil {
			x.Clear()
		}
		return err
	}}

	opMerge = operator{"merge", func(s State) error {
		var err error
		y := tryPopAs(&err, s, 1, AsArray)[0]
		x := tryPopAs(&err, s, 1, AsArray)[0]
		if err != nil {
			return err
		}
		z := new(LiteralArray)
		for _, v := range []Array{x, y} {
			switch v := v.(type) {
			case *LiteralArray:
				z.Append(*v...)
			case *ExecutableArray:
				z.Append(*v...)
			default:
				for i, n := 0, v.Len(); i < n; i++ {
					z.Append(v.Get(i))
				}
			}
		}
		tryPush(&err, s, z)
		return nil
	}}
)

type _opXStart struct{}

func (_opXStart) Type() Type     { return NameType }
func (_opXStart) String() string { return "{" }

func (_opXStart) Execute(s State) error {
	return s.Control().Push(&execArrayBuilder{frame: s.Data().Depth(), depth: 1})
}

type _opXStop struct{}

func (_opXStop) Type() Type     { return NameType }
func (_opXStop) String() string { return "}" }

type execArrayBuilder struct {
	frame int
	depth int
}

func (*execArrayBuilder) String() string { return "execArrayBuilder" }

func (a *execArrayBuilder) ExecuteModified(s State, v Value) (bool, error) {
	var r Value
	if n, ok := v.(Name); ok {
		if v, err := Resolve(s, n); err == nil {
			r, err = v.Load(s)
			if err != nil {
				return false, err
			}
		}
	}

	switch {
	case is[_opXStart](v), is[_opXStart](r):
		a.depth++
		return true, s.Data().Push(v)

	case is[_opXStop](v), is[_opXStop](r):
		a.depth--
		if a.depth > 0 {
			return true, s.Data().Push(v)
		}

	default:
		return true, s.Data().Push(v)
	}

	var err error
	u := tryPop(&err, s, s.Data().Depth()-a.frame)
	c := tryPopControl(&err, s, 1)
	if err == nil && c[0] != a {
		return true, errors.New("invalid control stack")
	}
	tryPush(&err, s, (*ExecutableArray)(&u))
	return true, err
}

func is[V any](v any) bool {
	_, ok := v.(V)
	return ok
}

func resolveArrayOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "[", "mark":
		return opAStart, true
	case "]", "arraytomark":
		return opAStop, true
	case "{":
		return opXStart, true
	case "}":
		return opXStop, true
	case "length":
		return opLength, true
	case "getat":
		return opGetAt, true
	case "newarray":
		return opNewArray, true
	case "memberof":
		return opMemberOf, true
	case "copyelements":
		return opCopyElem, true
	case "addto":
		return opAddTo, true
	case "addat":
		return opAddAt, true
	case "remove":
		return opRemove, true
	case "removeat":
		return opRemoveAt, true
	case "add_no_dups":
		return opAddUnique, true
	case "clear":
		return opClear, true
	case "merge":
		return opMerge, true
	}
	return nil, false
}

func arrayExecute(s State, v Value) error {
	switch v := v.(type) {
	case ArrayExecutable:
		return Execute(s, Function(v.ArrayExecute))
	}
	return Execute(s, v)
}

func equalTo(v Value) func(Value) bool {
	return func(u Value) bool {
		return EqualValues(v, u)
	}
}

func arrayToString(a []Value) string {
	s := make([]string, len(a))
	for i, v := range a {
		s[i] = v.String()
	}
	return strings.Join(s, " ")
}

type ptrValue[S any] interface {
	~*S
	Value
}

func popAsArray[T ptrValue[S], S ~[]Value](s State) error {
	v, err := PopDataFrame(s)
	if err != nil {
		return err
	}
	u := S(v)
	return s.Data().Push(T(&u))
}

func copyArray[T ptrValue[S], S ~[]Value](a T) T {
	b := make(S, len(*a))
	copy(b, *a)
	return T(&b)
}
