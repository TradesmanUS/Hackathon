package vm

import (
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
)

type Numeric interface {
	Value
	AsInt() (int64, error)
	AsUint() (uint64, error)
	AsFloat() (float64, error)
}

type Number[T constraints.Integer | constraints.Float] struct {
	value T
}

var _ Numeric = Number[int]{}
var _ Duration = Number[int]{}

func (v Number[T]) AsInt() (int64, error)              { return int64(v.value), nil }
func (v Number[T]) AsUint() (uint64, error)            { return uint64(v.value), nil }
func (v Number[T]) AsFloat() (float64, error)          { return float64(v.value), nil }
func (v Number[T]) AsDuration() (time.Duration, error) { return time.Duration(v.value), nil }

func (v Number[T]) String() string {
	d, ok := any(v.value).(time.Duration)
	if !ok || d < 24*time.Hour {
		return fmt.Sprint(v.value)
	}
	return formatDuration(d)
}

var (
	opZeq = binaryOp("z=", AsInt, func(x, y int64) Value { return boolean(x == y) })
	opZne = binaryOp("z≠", AsInt, func(x, y int64) Value { return boolean(x != y) })
	opZgt = binaryOp("z>", AsInt, func(x, y int64) Value { return boolean(x > y) })
	opZge = binaryOp("z≥", AsInt, func(x, y int64) Value { return boolean(x >= y) })
	opZlt = binaryOp("z<", AsInt, func(x, y int64) Value { return boolean(x < y) })
	opZle = binaryOp("z≤", AsInt, func(x, y int64) Value { return boolean(x <= y) })

	opZadd = binaryOp("z+", AsInt, func(x, y int64) Value { return Number[int64]{x + y} })
	opZsub = binaryOp("z-", AsInt, func(x, y int64) Value { return Number[int64]{x - y} })
	opZmul = binaryOp("z*", AsInt, func(x, y int64) Value { return Number[int64]{x * y} })
	opZdiv = binaryOp("z/", AsInt, func(x, y int64) Value { return Number[int64]{x / y} })
	opZabs = unaryOp("zabs", AsInt, func(x int64) Value { return Number[int64]{abs(x)} })
	opZneg = unaryOp("zneg", AsInt, func(x int64) Value { return Number[int64]{-x} })

	opFeq = binaryOp("f=", AsFloat, func(x, y float64) Value { return boolean(x == y) })
	opFne = binaryOp("f≠", AsFloat, func(x, y float64) Value { return boolean(x != y) })
	opFgt = binaryOp("f>", AsFloat, func(x, y float64) Value { return boolean(x > y) })
	opFge = binaryOp("f≥", AsFloat, func(x, y float64) Value { return boolean(x >= y) })
	opFlt = binaryOp("f<", AsFloat, func(x, y float64) Value { return boolean(x < y) })
	opFle = binaryOp("f≤", AsFloat, func(x, y float64) Value { return boolean(x <= y) })

	opFadd = binaryOp("f+", AsFloat, func(x, y float64) Value { return Number[float64]{x + y} })
	opFsub = binaryOp("f-", AsFloat, func(x, y float64) Value { return Number[float64]{x - y} })
	opFmul = binaryOp("f*", AsFloat, func(x, y float64) Value { return Number[float64]{x * y} })
	opFdiv = binaryOp("f/", AsFloat, func(x, y float64) Value { return Number[float64]{x / y} })
	opFabs = unaryOp("fabs", AsFloat, func(x float64) Value { return Number[float64]{abs(x)} })
	opFneg = unaryOp("fneg", AsFloat, func(x float64) Value { return Number[float64]{-x} })

	opRoundTo = operator{"roundTo", func(s State) error {
		var err error

		args := tryPop(&err, s, 3)
		value := tryAs(&err, args[0], AsFloat)
		places := tryAs(&err, args[1], AsInt)
		boundary := tryAs(&err, args[2], AsFloat)

		// If the boundary is zero (or negative), use the smallest possible
		// negative float so that any fractional value is
		if boundary <= 0 {
			// The smallest possible negative float
			boundary = math.Nextafter(0, -1)
		}
		switch {
		case boundary < 0:
			// If the boundary is zero (or negative), always round up
			boundary = 0

		case boundary > 1:
			// If the boundary is one or greater, always round down (truncate)
			boundary = 1

		default:
			// Shift the boundary one increment smaller so that r == boundary
			// rounds up
			boundary = math.Nextafter(boundary, 0)
		}

		tryPushResult(&err, s, func() (Value, error) {
			adj := math.Pow10(int(places))
			if value < 0 {
				adj = -adj
			}

			// Split the value into the integer part (Q) and fractional part (R)
			// after adjusting
			x := value * adj
			q := int64(x)
			r := x - float64(q)

			// Round?
			if math.Abs(r) > boundary {
				q++
			}

			// Reverse the adjustment
			return Number[float64]{float64(q) / adj}, nil
		})
		return err
	}}
)

func resolveNumericOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "eq", "==":
		return opZeq, true
	case "ne", "!=":
		return opZne, true
	case "gt", ">":
		return opZgt, true
	case "ge", ">=":
		return opZge, true
	case "lt", "<":
		return opZlt, true
	case "le", "<=":
		return opZle, true

	case "feq", "f==":
		return opFeq, true
	case "fne", "f!=":
		return opFne, true
	case "fgt", "f>":
		return opFgt, true
	case "fge", "f>=":
		return opFge, true
	case "flt", "f<":
		return opFlt, true
	case "fle", "f<=":
		return opFle, true

	case "add", "+":
		return opZadd, true
	case "sub", "-":
		return opZsub, true
	case "mul", "*":
		return opZmul, true
	case "div", "/":
		return opZdiv, true
	case "abs":
		return opZabs, true
	case "neg":
		return opZneg, true

	case "fadd", "f+":
		return opFadd, true
	case "fsub", "f-":
		return opFsub, true
	case "fmul", "f*":
		return opFmul, true
	case "fdiv", "f/":
		return opFdiv, true
	case "fabs":
		return opFabs, true
	case "fneg":
		return opFneg, true

	case "roundto":
		return opRoundTo, true
	}
	return nil, false
}

func abs[V constraints.Signed | constraints.Float](z V) V {
	if z < 0 {
		return -z
	}
	return z
}
