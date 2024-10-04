package vm

import (
	"errors"
	"io"
)

func Execute(s State, v ...Value) error {
	type Executable interface {
		Execute(State) error
	}

	type ExecutionModifier interface {
		ExecuteModified(State, Value) (bool, error)
	}

	for _, v := range v {
		if n := s.Control().Depth(); n > 0 {
			if em, ok := s.Control().Peek(n - 1).(ExecutionModifier); ok {
				ok, err := em.ExecuteModified(s, v)
				if err != nil {
					return err
				}
				if ok {
					continue
				}
			}
		}

		var err error
		switch v := v.(type) {
		case Executable:
			err = v.Execute(s)
		default:
			err = s.Data().Push(v)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func ExecuteString(s State, src string) error {
	return Execute(s, ExecutableString(src))
}

func Compile(src []byte) (Value, error) {
	n := new(Scanner)
	n.Init(src)
	var v ExecutableArray
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
	return &v, nil
}

func CompileString(src string) (Value, error) {
	return Compile([]byte(src))
}

func AsLiteral(v Value) (Value, error) {
	type CanBeLiteral interface {
		Value
		AsLiteral() (Value, error)
	}
	u, ok := v.(CanBeLiteral)
	if !ok {
		return u, nil
	}
	return u.AsLiteral()
}

func AsExecutable(v Value) (Value, error) {
	type CanBeExecutable interface {
		Value
		AsExecutable() (Value, error)
	}
	u, ok := v.(CanBeExecutable)
	if !ok {
		return v, nil
	}
	return u.AsExecutable()
}
