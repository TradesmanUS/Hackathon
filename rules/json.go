package main

import (
	"encoding/json"
	"fmt"

	"github.com/C3Rules/Go-DTRules/pkg/vm"
)

type jEntity struct {
	name   string
	values map[string]any
}

var _ vm.Entity = (*jEntity)(nil)

func (j *jEntity) Type() vm.Type      { return vm.ExternalType }
func (j *jEntity) String() string     { return mustMarshal(j.values) }
func (j *jEntity) EntityName() string { return j.name }

func (j *jEntity) Field(name vm.Name) (vm.Variable, bool) {
	if _, ok := j.values[name.Name()]; !ok {
		return nil, false
	}
	return &jVar{name.Name(), j.values}, true
}

type jVar struct {
	name   string
	values map[string]any
}

func (j *jVar) Load(vm.State) (vm.Value, error) {
	v, ok := j.values[j.name]
	if !ok {
		return vm.Null, nil
	}

	return j2vm(j.name, v)
}

func (j *jVar) Store(v vm.Value) error {
	u, err := vm.AsAny(v)
	if err != nil {
		return err
	}
	j.values[j.name] = u
	return nil
}

func j2vm(name string, v any) (vm.Value, error) {
	switch v := v.(type) {
	case map[string]any:
		return &jEntity{name, v}, nil

	case []any:
		// [vm.Array] is not friendly to errors, so convert all the values now
		a := make(vm.LiteralArray, len(v))
		for i, v := range v {
			v, err := vm.AsValue(v)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			a[i] = v
		}
		return &a, nil
	}

	return vm.AsValue(v)
}

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func getJsonField[V any](v any, key ...any) (V, error) {
	if len(key) == 0 {
		u, ok := v.(V)
		if !ok {
			return u, fmt.Errorf("bad type: want %T, got %T", u, v)
		}
		return u, nil
	}

	switch k := key[0].(type) {
	case string:
		x, ok := v.(map[string]any)
		if !ok {
			var z V
			return z, fmt.Errorf("bad type: want object, got %T", v)
		}

		v, ok = x[k]
		if !ok {
			var z V
			return z, fmt.Errorf("%q not found", k)
		}

		u, err := getJsonField[V](v, key[1:]...)
		if err != nil {
			var z V
			return z, fmt.Errorf("%s: %w", k, err)
		}
		return u, nil

	case func(any) bool:
		x, ok := v.([]any)
		if !ok {
			var z V
			return z, fmt.Errorf("bad type: want array, got %T", v)
		}

		for i, u := range x {
			if k(u) {
				w, err := getJsonField[V](u, key[1:]...)
				if err != nil {
					var z V
					return z, fmt.Errorf("[%d]: %w", i, err)
				}
				return w, nil
			}
		}

		var z V
		return z, fmt.Errorf("matching entry not found")

	default:
		panic("bad key")
	}

}
