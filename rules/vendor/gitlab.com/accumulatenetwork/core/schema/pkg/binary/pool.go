package binary

import "sync"

type Pool[T any] sync.Pool

func NewPool[T any](new func() T) *Pool[T] {
	return (*Pool[T])(&sync.Pool{New: func() any { return new() }})
}

func NewPointerPool[T any]() *Pool[*T] {
	return (*Pool[*T])(&sync.Pool{New: func() any { return new(T) }})
}

func (p *Pool[T]) Get() T {
	return (*sync.Pool)(p).Get().(T)
}

func (p *Pool[T]) Put(v T) {
	(*sync.Pool)(p).Put(v)
}
