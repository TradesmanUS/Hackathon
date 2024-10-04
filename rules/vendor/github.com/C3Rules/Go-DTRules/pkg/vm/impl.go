package vm

import (
	"context"
)

type impl struct {
	context context.Context
	entity  stack[Entity]
	value   stack[Value]
	control stack[Control]
}

var _ State = (*impl)(nil)

func (s *impl) Control() Stack[Control] { return &s.control }
func (s *impl) Data() Stack[Value]      { return &s.value }
func (s *impl) Entity() Stack[Entity]   { return &s.entity }

func (s *impl) Context() context.Context {
	if s.context == nil {
		s.context = context.Background()
	}
	return s.context
}

func (s *impl) SetContext(ctx context.Context) {
	s.context = ctx
}
