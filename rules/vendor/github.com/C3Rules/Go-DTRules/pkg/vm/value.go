package vm

import (
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/attribute"
)

var errValueIsNot = errors.New("value is not")
var errNotArray = fmt.Errorf("%w an array", errValueIsNot)
var errNotNumeric = fmt.Errorf("%w numeric", errValueIsNot)
var errNotBoolean = fmt.Errorf("%w boolean", errValueIsNot)
var errNotDateTime = fmt.Errorf("%w a date", errValueIsNot)
var errNotDuration = fmt.Errorf("%w a duration", errValueIsNot)

var Null null

type Value interface {
	Type() Type
	String() string
}

type Entity interface {
	EntityName() string
	Field(Name) (Variable, bool)
}

type Variable interface {
	Load(State) (Value, error)
	Store(Value) error
}

type Named struct {
	Name string
	Value
}

func (n *Named) String() string { return n.Name }

func (n *Named) Execute(s State) error {
	span, end := StartSpan(s, "ExecuteNamed")
	defer end()
	span.SetAttributes(
		attribute.String("Name", n.Name),
		attribute.String("Postscript", n.Value.String()))
	return Execute(s, n.Value)
}

type ReadOnlyVariable struct{ Value }

func (v ReadOnlyVariable) Load(State) (Value, error) { return v.Value, nil }
func (ReadOnlyVariable) Store(Value) error           { return errors.New("readonly") }

type null struct{}

func (null) String() string { return "" }

func ParseValue(src []byte) (Value, error) {
	n := new(Scanner)
	n.EvaluateLiteralArrays = false
	n.Init(src)
	var v LiteralArray
	for {
		u, err := n.Scan()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		v = append(v, u)
	}
	switch len(v) {
	case 0:
		return Null, nil
	case 1:
		return v[0], nil
	}
	return &v, nil
}

func ParseValueString(src string) (Value, error) {
	return ParseValue([]byte(src))
}

// EqualValues checks if A equals B, from the perspective of A. Thus,
// EqualValues may not be commutative depending on how A defines equality. If A
// defines a method `Equal(Value) bool`, EqualValues will call that method.
func EqualValues(a, b Value) bool {
	if a, ok := a.(interface{ Equal(Value) bool }); ok && a.Equal(b) {
		return true
	}
	if a == b {
		return true
	}

	switch a := a.(type) {
	case Name:
		if b, ok := b.(Name); ok {
			return a.Name() == b.Name()
		}
	case LiteralString, ExecutableString:
		if a.String() == b.String() {
			return true
		}
	case Number[uint], Number[uint8], Number[uint16], Number[uint32], Number[uint64]:
		if areEqual(a, b, AsUint) {
			return true
		}
	case Number[int], Number[int8], Number[int16], Number[int32], Number[int64]:
		if areEqual(a, b, AsInt) {
			return true
		}
	case Number[float32], Number[float64], Numeric:
		if areEqual(a, b, AsFloat) {
			return true
		}
	case Boolean:
		if areEqual(a, b, AsBool) {
			return true
		}
	case Array:
		b, ok := b.(Array)
		if !ok || a.Len() != b.Len() {
			return false
		}
		for i, n := 0, a.Len(); i < n; i++ {
			if !EqualValues(a.Get(i), b.Get(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func areEqual[V comparable](a, b Value, as func(Value) (V, error)) bool {
	c, e1 := as(a)
	d, e2 := as(b)
	return e1 == nil && e2 == nil && c == d
}
