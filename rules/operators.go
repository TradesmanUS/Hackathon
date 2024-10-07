package main

import (
	"errors"
	"strings"
	"time"

	"github.com/C3Rules/Go-DTRules/pkg/vm"
)

type ops struct{}

func (ops) EntityName() string { return "operators" }

func (ops) Field(name vm.Name) (vm.Variable, bool) {
	switch strings.ToLower(name.String()) {
	case "parsedate":
		return op{vm.Function(parseDate)}, true

	default:
		return nil, false
	}
}

func parseDate(s vm.State) error {
	str, err := vm.PopAs(s, 1, vm.AsString)
	if err != nil {
		return err
	}

	d, err := time.Parse("1/2/2006", str[0])
	if err != nil {
		return err
	}

	v, err := vm.AsValue(d)
	if err != nil {
		return err
	}

	return s.Data().Push(v)
}

type op struct {
	v vm.Value
}

func (o op) Load(vm.State) (vm.Value, error) { return o.v, nil }
func (op) Store(vm.Value) error              { return errors.New("read only") }
