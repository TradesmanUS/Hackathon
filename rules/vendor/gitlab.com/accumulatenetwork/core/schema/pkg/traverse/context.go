package traverse

import "reflect"

type Context[V comparable] struct {
	stack    []valPtr[V]
	values   map[V]bool
	pointers map[uintptr]bool
}

type valPtr[V any] struct {
	val V
	ptr uintptr
}

func (c *Context[V]) Depth() int {
	return len(c.stack)
}

func (c *Context[V]) Get(i int) V {
	return c.stack[len(c.stack)-1-i].val
}

func (c *Context[V]) Seen(v V) bool {
	ptr, ok := Pointer(v)
	if ok {
		_, ok = c.pointers[ptr]
	} else {
		_, ok = c.values[v]
	}
	return ok
}

func (c *Context[V]) Mark(v V) (ptr uintptr, seen bool) {
	seen = c.Seen(v)
	ptr, _ = Pointer(v)
	c.mark(v, ptr, false)
	return ptr, seen
}

func (c *Context[V]) mark(v V, ptr uintptr, ok bool) {
	if c.values == nil {
		c.values = map[V]bool{}
		c.pointers = map[uintptr]bool{}
	}

	c.values[v] = ok
	if ptr != 0 {
		c.pointers[ptr] = ok
	}
}

func (c *Context[V]) Push(v V) bool {
	// Check for duplicates
	ptr, ok := Pointer(v)
	if ok {
		ok = c.pointers[ptr]
	} else {
		ok = c.values[v]
	}
	if ok {
		return false
	}

	c.stack = append(c.stack, valPtr[V]{v, ptr})
	c.mark(v, ptr, true)
	return true
}

func (c *Context[V]) Pop(_ V) {
	i := len(c.stack) - 1
	u := c.stack[i]
	c.stack[i] = valPtr[V]{}
	c.stack = c.stack[:i]
	c.mark(u.val, u.ptr, false)
}

func Pointer(v any) (uintptr, bool) {
	rv, ok := v.(reflect.Value)
	if !ok {
		rv = reflect.ValueOf(v)
	}
	if rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map,
		reflect.Pointer,
		reflect.Slice:
		return rv.Pointer(), true

	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.String,
		reflect.Struct:
		if !rv.CanAddr() {
			return 0, false
		}
		return rv.Addr().Pointer(), true

	default:
		return 0, false
	}
}
