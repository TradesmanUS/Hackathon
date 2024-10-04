package dt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/C3Rules/Go-DTRules/pkg/vm"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type EntityDefinition map[string]EntityDefinitionField

type EntityDefinitionField struct {
	Type     vm.Type
	Default  vm.Value
	Writable bool
	Required bool
}

type Entity struct {
	name   string
	fields map[string]*EntityField
}

type EntityField struct {
	Name     string
	Entity   *Entity
	Type     vm.Type
	Value    vm.Value
	Writable bool
	Required bool
}

type UndefinedField struct {
	Field *EntityField
}

func (u *UndefinedField) Error() string {
	return fmt.Sprintf("required field %q has no value", u.Field.Name)
}

var _ vm.Entity = (*Entity)(nil)

func (ed EntityDefinition) New(name string) *Entity {
	e := new(Entity)
	e.name = name
	e.fields = make(map[string]*EntityField, len(ed))
	for name, f := range ed {
		e.fields[strings.ToLower(name)] = &EntityField{name, e, f.Type, f.Default, f.Writable, f.Required}
	}
	return e
}

func (e *Entity) Type() vm.Type      { return EntityType }
func (e *Entity) String() string     { return e.name }
func (e *Entity) EntityName() string { return e.name }

func (e *Entity) Field(key vm.Name) (vm.Variable, bool) {
	f, ok := e.fields[strings.ToLower(key.Name())]
	if !ok {
		return nil, false
	}
	return f, ok
}

func (f *EntityField) Load(s vm.State) (vm.Value, error) {
	if f.Required && (f.Value == nil || f.Value.Type() == vm.NullType) {
		span := trace.SpanFromContext(s.Context())
		span.AddEvent("UndefinedField",
			trace.WithAttributes(
				attribute.String("Name", f.Name)))
		return nil, &UndefinedField{Field: f}
	}
	return f.Value, nil
}

func (f *EntityField) Store(value vm.Value) error {
	if !f.Writable {
		return errors.New("readonly")
	}
	if f.Type != vm.NullType && f.Type != value.Type() {
		return fmt.Errorf("wrong type: want %v, got %v", f.Type, value.Type())
	}
	f.Value = value
	return nil
}

func (e *Entity) Set(name string, value any) error {
	f, ok := e.fields[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("%v is not a field of this entity", name)
	}
	v, err := vm.AsValue(value)
	if err != nil {
		return err
	}
	switch {
	case f.Type == vm.NullType:
		f.Value = v
	case f.Type == v.Type():
		f.Value = v
	case f.Type == vm.ArrayType && v.Type() != vm.ArrayType:
		if a, ok := f.Value.(vm.Array); ok {
			a.Append(v)
		} else {
			f.Value = &vm.LiteralArray{v}
		}
	default:
		return fmt.Errorf("wrong type: want %v, got %v", f.Type, v.Type())
	}
	return nil
}

func (e *Entity) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.fields)
}

func (e *EntityField) MarshalJSON() ([]byte, error) {
	if e.Value.Type() == vm.NullType {
		return json.Marshal(nil)
	}
	return json.Marshal(e.Value)
}
