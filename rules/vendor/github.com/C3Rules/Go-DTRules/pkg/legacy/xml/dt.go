package xml

import (
	"fmt"
	"strings"

	"github.com/C3Rules/Go-DTRules/pkg/dt"
	"github.com/C3Rules/Go-DTRules/pkg/vm"
)

type DT struct {
	Slice[*DecisionTable]
}

type DecisionTable struct {
	TableName        string                  `xml:"table_name"`
	XlsFile          string                  `xml:"xls_file"`
	AttributeFields  Dictionary              `xml:"attribute_fields"`
	Contexts         Slice[*Context]         `xml:"contexts"`
	InitialActions   Slice[*InitialAction]   `xml:"initial_actions"`
	Conditions       Slice[*Condition]       `xml:"conditions"`
	Actions          Slice[*Action]          `xml:"actions"`
	PolicyStatements Slice[*PolicyStatement] `xml:"policy_statements"`
}

type Context struct {
	Number      int    `xml:"context_number"`
	Comment     string `xml:"context_comment"`
	Description string `xml:"context_description"`
	Postfix     string `xml:"context_postfix"`
}

type InitialAction struct {
	Number      int    `xml:"initial_action_number"`
	Comment     string `xml:"initial_action_comment"`
	Requirement string `xml:"initial_action_requirement"`
	Description string `xml:"initial_action_description"`
	Postfix     string `xml:"initial_action_postfix"`
}

type Condition struct {
	Number      int       `xml:"condition_number"`
	Comment     string    `xml:"condition_comment"`
	Requirement string    `xml:"condition_requirement"`
	Description string    `xml:"condition_description"`
	Postfix     string    `xml:"condition_postfix"`
	Columns     []*Column `xml:"condition_column"`
}

type Action struct {
	Number      int       `xml:"action_number"`
	Comment     string    `xml:"action_comment"`
	Requirement string    `xml:"action_requirement"`
	Description string    `xml:"action_description"`
	Postfix     string    `xml:"action_postfix"`
	Columns     []*Column `xml:"action_column"`
}

type Column struct {
	Number int    `xml:"column_number,attr"`
	Value  string `xml:"column_value,attr"`
}

type PolicyStatement struct {
	Column      int    `xml:"column,attr"`
	Description string `xml:"policy_description"`
	Statement   string `xml:"policy_statement_postfix"`
}

func (x DT) Compile() (vm.Entity, error) {
	tables := TablesEntity{}
	for _, table := range x.Slice {
		v, err := table.Compile()
		if err != nil {
			return nil, err
		}
		tables[strings.ToLower(table.TableName)] = vm.ReadOnlyVariable{Value: v}
	}
	return tables, nil
}

func (x *DecisionTable) Compile() (vm.Value, error) {
	t := new(dt.DecisionTable)

	// Compile statements
	var err error
	for i, x := range x.InitialActions {
		t.Before = append(t.Before, tryCompile(&err, x.Postfix, "initial action %d", i))
	}
	for i, x := range x.Conditions {
		t.Conditions = append(t.Conditions, tryCompile(&err, x.Postfix, "condition %d", i))
	}
	for i, x := range x.Actions {
		t.Actions = append(t.Actions, tryCompile(&err, x.Postfix, "action %d", i))
	}

	// Compile context
	var v vm.Value = t
	if len(x.Contexts) > 0 {
		v = &vm.ExecutableArray{v}
	}
	for i := len(x.Contexts) - 1; i >= 0; i-- {
		switch u := tryCompile(&err, x.Contexts[i].Postfix, "context %d", i).(type) {
		case vm.Array:
			w := vm.ExecutableArray{v}
			for i, n := 0, u.Len(); i < n; i++ {
				w = append(w, u.Get(i))
			}
			v = (*vm.ExecutableArray)(&w)
		default:
			v = &vm.ExecutableArray{v, u}
		}
	}
	if err != nil {
		return nil, err
	}

	// Build cases
	cases := map[int]*dt.Case{}
	getCase := func(i int) *dt.Case {
		if c, ok := cases[i]; ok {
			return c
		}
		c := &dt.Case{}
		cases[i] = c
		t.Cases = append(t.Cases, c)
		c.Conditions = make([]dt.CaseCondition, len(x.Conditions))
		return c
	}
	for i, x := range x.Conditions {
		for _, col := range x.Columns {
			switch col.Value {
			case "Y":
				getCase(col.Number - 1).Conditions[i] = dt.True
			case "N":
				getCase(col.Number - 1).Conditions[i] = dt.False
			case "*":
				getCase(col.Number - 1).Always = true
			default:
				return nil, fmt.Errorf("unknown condition column value %q", col.Value)
			}
		}
	}
	for i, x := range x.Actions {
		for _, col := range x.Columns {
			c := getCase(col.Number - 1)
			switch col.Value {
			case "X":
				c.Actions = append(c.Actions, i)
			default:
				return nil, fmt.Errorf("unknown action column value %q", col.Value)
			}
		}
	}

	for i, c := range t.Cases {
		if c.Always && len(c.Conditions) > 0 {
			return nil, fmt.Errorf("column %d has both always (*) and conditions", i)
		}
	}

	return &vm.Named{Name: x.TableName, Value: v}, nil
}

func tryCompile(err *error, src, context string, args ...any) vm.Value {
	if *err != nil {
		return nil
	}
	var v vm.Value
	v, *err = vm.CompileString(src)
	if *err != nil {
		*err = fmt.Errorf("%s: compile string: %w", fmt.Sprintf(context, args...), *err)
		return nil
	}
	v, *err = vm.AsExecutable(v)
	if *err != nil {
		*err = fmt.Errorf("%s: as executable: %w", fmt.Sprintf(context, args...), *err)
		return nil
	}
	return v
}

type TablesEntity map[string]vm.ReadOnlyVariable

func (e TablesEntity) Type() vm.Type      { return dt.EntityType }
func (e TablesEntity) String() string     { return "DecisionTables" }
func (e TablesEntity) EntityName() string { return "DecisionTables" }

func (e TablesEntity) Field(key vm.Name) (vm.Variable, bool) {
	v, ok := e[strings.ToLower(key.Name())]
	if !ok {
		return nil, false
	}
	return v, ok
}
