package rules

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

type jQuery interface {
	Query(any) (any, error)
	Errorf(err error) error
}

type objectField string

func (q objectField) Errorf(err error) error { return fmt.Errorf("%s: %w", q, err) }

func (q objectField) Query(v any) (any, error) {
	x, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("bad type: want object, got %T", v)
	}

	v, ok = x[string(q)]
	if !ok {
		return nil, fmt.Errorf("%q not found", q)
	}
	return v, nil
}

func findValue[V comparable](target V) arrayPredicate {
	return arrayPredicate(func(v any) (bool, error) {
		u, err := getJsonField[V](v)
		if err != nil {
			return false, err
		}
		return u == target, nil
	})
}

type arrayPredicate func(any) (bool, error)

func (q arrayPredicate) Errorf(err error) error { return fmt.Errorf("[?]: %w", err) }

func (q arrayPredicate) Query(v any) (any, error) {
	x, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("bad type: want array, got %T", v)
	}

	for _, u := range x {
		ok, err := q(u)
		if err != nil {
			return nil, err
		}
		if ok {
			return u, nil
		}
	}

	return nil, fmt.Errorf("matching entry not found")
}

func (q arrayPredicate) For(scope ...jQuery) arrayPredicate {
	return func(v any) (bool, error) {
		v, err := compoundQuery(scope).Query(v)
		if err != nil {
			return false, err
		}
		return q(v)
	}
}

type compoundQuery []jQuery

func (q compoundQuery) Errorf(err error) error { return err }

func (q compoundQuery) Query(v any) (_ any, err error) {
	for _, query := range q {
		v, err = query.Query(v)
		if err != nil {
			return nil, err
		}
		defer func(q jQuery) {
			if err != nil {
				err = q.Errorf(err)
			}
		}(query)
	}
	return v, nil
}

func getJsonField[V any](v any, query ...jQuery) (V, error) {
	v, err := compoundQuery(query).Query(v)
	if err != nil {
		var z V
		return z, err
	}

	u, ok := v.(V)
	if !ok {
		return u, fmt.Errorf("bad type: want %T, got %T", u, v)
	}
	return u, nil
}
