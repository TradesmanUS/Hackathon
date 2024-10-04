package vm

import (
	"fmt"
	"strings"
)

var True = boolean(true)
var False = boolean(false)

type Boolean interface {
	Value
	AsBool() (bool, error)
}

type boolean bool

var _ Boolean = boolean(false)

func (v boolean) String() string         { return fmt.Sprint(bool(v)) }
func (v boolean) AsBool() (bool, error)  { return bool(v), nil }

var (
	opNot = unaryOp("not", AsBool, func(x bool) Value { return boolean(!x) })
	opAnd = binaryOp("and", AsBool, func(x, y bool) Value { return boolean(x && y) })
	opOr  = binaryOp("or", AsBool, func(x, y bool) Value { return boolean(x || y) })

	opBeq = binaryOp("b=", AsBool, func(x, y bool) Value { return boolean(x == y) })
	opBne = binaryOp("b≠", AsBool, func(x, y bool) Value { return boolean(x != y) })
)

func resolveBooleanOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "true":
		return boolean(true), true
	case "false":
		return boolean(false), true

	case "not", "!":
		return opNot, true
	case "and", "&&":
		return opAnd, true
	case "or", "||":
		return opOr, true

	case "beq", "b==":
		return opBeq, true
	case "bne", "b!=":
		return opBne, true
	}
	return nil, false
}
