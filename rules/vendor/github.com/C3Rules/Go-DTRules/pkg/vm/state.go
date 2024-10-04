package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errCannotResolve = errors.New("cannot resolve")

type State interface {
	Context() context.Context
	SetContext(context.Context)

	Control() Stack[Control]
	Data() Stack[Value]
	Entity() Stack[Entity]
}

func New() State { return new(impl) }

func PushDataFrame(s State) error {
	return s.Control().Push(dataFrame(s.Data().Depth()))
}

func PushEntityFrame(s State) error {
	return s.Control().Push(entityFrame(s.Entity().Depth()))
}

func PopDataFrame(s State) ([]Value, error) {
	return popf[dataFrame](s, s.Data())
}

func PopEntityFrame(s State) ([]Entity, error) {
	return popf[entityFrame](s, s.Entity())
}

func Resolve(s State, name Name) (Variable, error) {
	var entity Name
	orig := name
	if c, ok := name.(CompoundName); ok {
		entity = c.Entity
		name = c.Member
	}
	for i := s.Entity().Depth() - 1; i >= 0; i-- {
		e := s.Entity().Peek(i)
		if entity != nil && !strings.EqualFold(e.EntityName(), entity.Name()) {
			continue
		}
		v, ok := e.Field(name)
		if ok {
			return v, nil
		}
	}
	v, ok := resolveOperator(name.Name())
	if ok {
		return ReadOnlyVariable{v}, nil
	}
	return nil, fmt.Errorf("%w name %q", errCannotResolve, orig.Name())
}
