package vm

import (
	"fmt"
)

func tryResolve(ptr *error, s State, name Name) Variable {
	if *ptr != nil {
		return nil
	}
	var v Variable
	v, *ptr = Resolve(s, name)
	return v
}

func tryPop(ptr *error, s State, n int) []Value {
	if *ptr != nil {
		return make([]Value, n)
	}
	v, err := s.Data().Pop(n)
	if err != nil {
		*ptr = err
		return make([]Value, n)
	}
	return v
}

func tryPopControl(ptr *error, s State, n int) []Control {
	if *ptr != nil {
		return make([]Control, n)
	}
	c, err := s.Control().Pop(n)
	if err != nil {
		*ptr = err
		return make([]Control, n)
	}
	return c
}

func tryPopEntity(ptr *error, s State, n int) []Entity {
	if *ptr != nil {
		return make([]Entity, n)
	}
	v, err := s.Entity().Pop(n)
	if err != nil {
		*ptr = err
		return make([]Entity, n)
	}
	return v
}

func tryPopAs[V any](ptr *error, s State, n int, as func(Value) (V, error)) []V {
	u := make([]V, n)
	for i, v := range tryPop(ptr, s, n) {
		if *ptr != nil {
			break
		}
		u[i], *ptr = as(v)
	}
	return u
}

func tryAs[V any](ptr *error, v Value, as func(Value) (V, error)) V {
	var u V
	if *ptr != nil {
		return u
	}
	u, *ptr = as(v)
	return u
}

func tryPush(ptr *error, s State, v ...Value) {
	if *ptr != nil {
		return
	}
	*ptr = s.Data().Push(v...)
}

func tryPushControl(ptr *error, s State, c ...Control) {
	if *ptr != nil {
		return
	}
	*ptr = s.Control().Push(c...)
}

func tryPushResult(ptr *error, s State, fn func() (Value, error)) {
	if *ptr != nil {
		return
	}
	var r Value
	r, *ptr = fn()
	tryPush(ptr, s, r)
}

func tryPushResult1[X any](ptr *error, s State, fn func(X) Value, x X) {
	if *ptr != nil {
		return
	}
	*ptr = s.Data().Push(fn(x))
}

func tryPushResult2[X, Y any](ptr *error, s State, fn func(X, Y) Value, x X, y Y) {
	if *ptr != nil {
		return
	}
	*ptr = s.Data().Push(fn(x, y))
}

func tryPushResult3[X, Y, Z any](ptr *error, s State, fn func(X, Y, Z) Value, x X, y Y, z Z) {
	if *ptr != nil {
		return
	}
	*ptr = s.Data().Push(fn(x, y, z))
}

func tryExec(ptr *error, s State, v Value) {
	if *ptr != nil {
		return
	}
	*ptr = Execute(s, v)
}

func tryExecPopAs[V any](ptr *error, s State, v Value, n int, as func(Value) (V, error)) []V {
	u := make([]V, n)
	if *ptr != nil {
		return u
	}
	var r []Value
	r, *ptr = ExecuteFramed(s, v)
	if len(r) < n {
		*ptr = fmt.Errorf("expected %d results, got %d", n, len(r))
		return u
	}
	if len(r) > n {
		u = append(u, make([]V, len(r)-n)...)
	}
	for i, r := range r {
		u[i], *ptr = as(r)
	}
	return u
}

func tryCall[V any](ptr *error, fn func() V) V {
	if *ptr != nil {
		var z V
		return z
	}
	return fn()
}

func tryPushEntity(ptr *error, s State, v ...Entity) {
	if *ptr != nil {
		return
	}
	*ptr = s.Entity().Push(v...)
}
