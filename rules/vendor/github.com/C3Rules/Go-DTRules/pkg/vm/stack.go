package vm

import "errors"

var errStackUnderflow = errors.New("stack underflow")

type Stack[V any] interface {
	Depth() int
	Push(...V) error
	Pop(int) ([]V, error)

	// Peek returns the Ith value of the stack. Peek may panic if I is out of
	// bounds.
	Peek(i int) V
}

type stack[V any] []V

func (s stack[V]) Depth() int   { return len(s) }
func (s stack[V]) Peek(i int) V { return s[i] }

func (s *stack[V]) Push(v ...V) error {
	*s = append(*s, v...)
	return nil
}

func (s *stack[V]) Pop(n int) ([]V, error) {
	if len(*s) < n {
		return nil, errStackUnderflow
	}
	i := len(*s) - n
	v := make([]V, n)
	copy(v, (*s)[i:])
	*s = (*s)[:i]
	return v, nil
}

func popf[F ~int, S Stack[V], V any](s State, t S) ([]V, error) {
	v, err := s.Control().Pop(1)
	if err != nil {
		return nil, err
	}
	f, ok := v[0].(F)
	if !ok {
		return nil, errors.New("no stack frame")
	}
	if int(f) > t.Depth() {
		return nil, errors.New("invalid stack frame")
	}
	return t.Pop(t.Depth() - int(f))
}
