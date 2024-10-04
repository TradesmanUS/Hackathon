package vm

import (
	"errors"
	"strings"
)

var ErrExecutedControl = errors.New("tried to execute a control token")

type Control interface {
	String() string
}

type ControlPusher interface {
	Control
	Push(State) error
}

type dataFrame int

func (dataFrame) String() string { return "stackFrame" }

type entityFrame int

func (entityFrame) String() string { return "entityFrame" }

type loopCounter int

func (c *loopCounter) String() string     { return "loopCounter" }
func (c *loopCounter) Push(s State) error { return s.Data().Push(Number[loopCounter]{*c}) }

type Function func(State) error

func (Function) Name() string             { return "function" }
func (Function) String() string           { return "function" }
func (fn Function) Execute(s State) error { return fn(s) }

var (
	opCntrI = counterOp("I", 0)
	opCntrJ = counterOp("J", 1)
	opCntrK = counterOp("K", 2)

	opIf = operator{"if", func(s State) error {
		var err error
		test := tryPopAs(&err, s, 1, AsBool)[0]
		body := tryPop(&err, s, 1)[0]
		if test {
			tryExec(&err, s, body)
		}
		return err
	}}

	opIfElse = operator{"ifElse", func(s State) error {
		var err error
		test := tryPopAs(&err, s, 1, AsBool)[0]
		body := tryPop(&err, s, 2)
		if test {
			tryExec(&err, s, body[0])
		} else if err == nil {
			tryExec(&err, s, body[1])
		}
		return err
	}}

	opWhile = operator{"while", func(s State) error {
		var err error
		test := tryPop(&err, s, 1)[0]
		body := tryPop(&err, s, 1)[0]
		var i = 0
		tryPushControl(&err, s, (*loopCounter)(&i))
		for ; tryExecPopAs(&err, s, test, 1, AsBool)[0]; i++ {
			tryExec(&err, s, body)
		}
		tryPopControl(&err, s, 1)
		return err
	}}

	opFor = operator{"for", func(s State) error {
		var err error
		list := tryPopAs(&err, s, 1, AsArray)[0]
		body := tryPop(&err, s, 1)[0]
		var i = 0
		tryPushControl(&err, s, (*loopCounter)(&i))
		for ; i < tryCall(&err, list.Len) && err == nil; i++ {
			tryPush(&err, s, list.Get(i))
			tryExec(&err, s, body)
		}
		tryPopControl(&err, s, 1)
		return err
	}}

	opForRev = operator{"forRev", func(s State) error {
		var err error
		list := tryPopAs(&err, s, 1, AsArray)[0]
		body := tryPop(&err, s, 1)[0]
		i := tryCall(&err, list.Len) - 1
		tryPushControl(&err, s, (*loopCounter)(&i))
		for ; i >= 0 && err == nil; i-- {
			if i >= tryCall(&err, list.Len) {
				continue
			}
			tryPush(&err, s, list.Get(i))
			tryExec(&err, s, body)
		}
		tryPopControl(&err, s, 1)
		return err
	}}

	opForAll = operator{"forAll", func(s State) error {
		var err error
		list := tryPopAs(&err, s, 1, AsArray)[0]
		body := tryPop(&err, s, 1)[0]
		if err != nil {
			return err
		}
		var i = 0
		tryPushControl(&err, s, (*loopCounter)(&i))
		for ; i < tryCall(&err, list.Len) && err == nil; i++ {
			v := list.Get(i)
			if v == Null {
				continue
			}
			tryPushEntity(&err, s, tryAs(&err, v, as[Entity]))
			tryExec(&err, s, body)
			tryPopEntity(&err, s, 1)
		}
		tryPopControl(&err, s, 1)
		return err
	}}

	opForAllRev = operator{"forAllRev", func(s State) error {
		var err error
		list := tryPopAs(&err, s, 1, AsArray)[0]
		body := tryPop(&err, s, 1)[0]
		i := tryCall(&err, list.Len) - 1
		tryPushControl(&err, s, (*loopCounter)(&i))
		for ; i >= 0 && err == nil; i-- {
			if i >= tryCall(&err, list.Len) {
				continue
			}
			v := list.Get(i)
			if v == Null {
				continue
			}
			tryPushEntity(&err, s, tryAs(&err, v, as[Entity]))
			tryExec(&err, s, body)
			tryPopEntity(&err, s, 1)
		}
		tryPopControl(&err, s, 1)
		return err
	}}
)

func counterOp(name string, depth int) operator {
	return operator{name, func(s State) error {
		print("")
		depth := depth
		for i := s.Control().Depth() - 1; i >= 0; i-- {
			c, ok := s.Control().Peek(i).(ControlPusher)
			if !ok {
				continue
			}
			if depth > 0 {
				depth--
				continue
			}
			return c.Push(s)
		}
		return s.Data().Push(Null)
	}}
}

func resolveControlOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "i":
		return opCntrI, true
	case "j":
		return opCntrJ, true
	case "k":
		return opCntrK, true
	case "if":
		return opIf, true
	case "ifelse":
		return opIfElse, true
	case "while":
		return opWhile, true
	case "for":
		return opFor, true
	case "forr":
		return opForRev, true
	case "forall":
		return opForAll, true
	case "forallr":
		return opForAllRev, true
	}
	return nil, false
}
