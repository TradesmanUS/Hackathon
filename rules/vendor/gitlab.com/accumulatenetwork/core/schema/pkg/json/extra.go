package json

import (
	"encoding/json"
)

type Extra []Field

type Field struct {
	Name  string
	Value json.RawMessage
}

func (e *Extra) add(name string, value json.RawMessage) {
	*e = append(*e, Field{name, value})
}

type extraStack []*Extra

func (s *extraStack) push() {
	*s = append(*s, nil)
}

func (s *extraStack) pop() {
	i := len(*s) - 1
	(*s)[i] = nil
	*s = (*s)[:i]
}

func (s *extraStack) current() *Extra {
	i := len(*s) - 1
	if (*s)[i] != nil {
		return (*s)[i]
	}

	e := new(Extra)
	(*s)[i] = e
	return e
}
