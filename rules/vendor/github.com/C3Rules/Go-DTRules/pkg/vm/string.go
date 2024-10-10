package vm

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

type LiteralString string

func (v LiteralString) String() string                     { return string(v) }
func (v LiteralString) AsInt() (int64, error)              { return strconv.ParseInt(string(v), 10, 64) }
func (v LiteralString) AsUint() (uint64, error)            { return strconv.ParseUint(string(v), 10, 64) }
func (v LiteralString) AsFloat() (float64, error)          { return strconv.ParseFloat(string(v), 10) }
func (v LiteralString) AsBool() (bool, error)              { return strconv.ParseBool(string(v)) }
func (v LiteralString) AsDateTime() (time.Time, error)     { return parseDateTime(string(v)) }
func (v LiteralString) AsDuration() (time.Duration, error) { return parseDuration(string(v)) }
func (v LiteralString) AsExecutable() (Value, error)       { return ExecutableString(v), nil }

type ExecutableString string

func (v ExecutableString) String() string                     { return string(v) }
func (v ExecutableString) AsInt() (int64, error)              { return strconv.ParseInt(string(v), 10, 64) }
func (v ExecutableString) AsUint() (uint64, error)            { return strconv.ParseUint(string(v), 10, 64) }
func (v ExecutableString) AsFloat() (float64, error)          { return strconv.ParseFloat(string(v), 10) }
func (v ExecutableString) AsBool() (bool, error)              { return strconv.ParseBool(string(v)) }
func (v ExecutableString) AsDateTime() (time.Time, error)     { return parseDateTime(string(v)) }
func (v ExecutableString) AsDuration() (time.Duration, error) { return parseDuration(string(v)) }
func (v ExecutableString) AsLiteral() (Value, error)          { return LiteralString(v), nil }

func (v ExecutableString) Execute(s State) error {
	n := new(Scanner)
	n.Init([]byte(v))
	for {
		v, err := n.Scan()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		err = arrayExecute(s, v)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	opSlen = unaryOp("slen", AsString, func(x string) Value { return Number[int]{len(x)} })
	opSadd = binaryOp("s+", AsString, func(x, y string) Value { return LiteralString(x + y) })
	opSsub = binaryOp("s-", AsString, func(x, y string) Value { return LiteralString(strings.Replace(x, y, "", 1)) })

	opSeq = binaryOp("s=", AsString, func(x, y string) Value { return boolean(x == y) })
	opSne = binaryOp("s≠", AsString, func(x, y string) Value { return boolean(x != y) })
	opSgt = binaryOp("s>", AsString, func(x, y string) Value { return boolean(x > y) })
	opSge = binaryOp("s≥", AsString, func(x, y string) Value { return boolean(x >= y) })
	opSlt = binaryOp("s<", AsString, func(x, y string) Value { return boolean(x < y) })
	opSle = binaryOp("s≤", AsString, func(x, y string) Value { return boolean(x <= y) })

	opSeqic = binaryOp("sc=", AsString, func(x, y string) Value { return boolean(strings.EqualFold(x, y)) })
)

func resolveStringOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "strlength":
		return opSlen, true
	case "strconcat", "s+":
		return opSadd, true
	case "strremove", "s-":
		return opSsub, true

	case "streq", "s==":
		return opSeq, true
	case "streqignorecase", "sic==":
		return opSeq, true
	case "strne", "s!=":
		return opSne, true
	case "strgt", "s>":
		return opSgt, true
	case "strge", "s>=":
		return opSge, true
	case "strlt", "s<":
		return opSlt, true
	case "strle", "s<=":
		return opSle, true
	}
	return nil, false
}
