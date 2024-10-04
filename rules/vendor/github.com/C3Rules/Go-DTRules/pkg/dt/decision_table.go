package dt

import (
	"fmt"

	"github.com/C3Rules/Go-DTRules/pkg/vm"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DecisionTable struct {
	Mode       Mode
	Before     []vm.Value
	Conditions []vm.Value
	Actions    []vm.Value
	Cases      []*Case
}

type Mode int
type CaseCondition int

const (
	ModeFirst Mode = iota
	ModeAll
)

const (
	DontCare CaseCondition = iota
	True
	False
)

type Case struct {
	Always     bool
	Conditions []CaseCondition
	Actions    []int
}

type execution struct {
	vm.State
	*DecisionTable
	condResult []*bool
	span       trace.Span
}

func (d *DecisionTable) Type() vm.Type  { return DecisionTableType }
func (d *DecisionTable) String() string { return "decisionTable" }

func (d *DecisionTable) Execute(s vm.State) error {
	span, end := vm.StartSpan(s, "ExecuteTable")
	defer end()

	e := &execution{
		State:         s,
		DecisionTable: d,
		condResult:    make([]*bool, len(d.Conditions)),
		span:          span,
	}

	// Push a stack frame
	err := vm.PushDataFrame(s)
	if err != nil {
		return err
	}
	err = vm.PushEntityFrame(s)
	if err != nil {
		return err
	}

	// Execute before scripts
	for _, b := range d.Before {
		err = vm.Execute(s, b)
		if err != nil {
			return err
		}
	}

	// Execute each/the first case
	for i, c := range d.Cases {
		ok, err := e.executeCase(i, c)
		if err != nil {
			return fmt.Errorf("execute case %d: %w", i, err)
		}
		if ok && d.Mode == ModeFirst {
			break
		}
	}

	// Pop the stack frame
	_, err = vm.PopEntityFrame(s)
	if err != nil {
		return err
	}
	_, err = vm.PopDataFrame(s)
	if err != nil {
		return err
	}

	return nil
}

func (e *execution) executeCase(i int, c *Case) (bool, error) {
	if len(c.Conditions) > len(e.Conditions) {
		return false, fmt.Errorf("case has too many conditions")
	}

	// Check conditions
	ok, err := e.executeConditions(c)
	if !ok || err != nil {
		return false, err
	}

	span, end := vm.StartSpan(e, "ExecuteCase")
	defer end()
	span.SetAttributes(
		attribute.Int("Number", i))

	// Execute actions
	for _, i := range c.Actions {
		err := e.executeAction(i)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (e *execution) executeConditions(c *Case) (bool, error) {
	if c.Always {
		return true, nil
	}

	// Check conditions
	for i, c := range c.Conditions {
		if c == DontCare {
			continue
		}
		v, err := e.executeCondition(i)
		if err != nil {
			return false, fmt.Errorf("condition %d: %w", i, err)
		}
		switch c {
		case True:
			if !v {
				return false, nil
			}
		case False:
			if v {
				return false, nil
			}
		}
	}
	return true, nil
}

func (e *execution) executeCondition(i int) (bool, error) {
	if e.condResult[i] != nil {
		return *e.condResult[i], nil
	}

	span, end := vm.StartSpan(e, "ExecuteCondition")
	defer end()
	span.SetAttributes(
		attribute.Int("Number", i),
		attribute.String("Postscript", e.Conditions[i].String()))

	v, err := vm.ExecuteFramed(e.State, e.Conditions[i])
	if err != nil {
		return false, err
	}
	if len(v) == 0 {
		return false, fmt.Errorf("no result")
	}

	u := v[len(v)-1]
	ok, err := vm.AsBool(u)
	if err != nil {
		return false, err
	}

	if u.Type() == vm.NullType {
		span.SetAttributes(
			attribute.String("Result", "null"))
	} else {
		span.SetAttributes(
			attribute.Bool("Result", ok))
	}

	e.condResult[i] = &ok
	return ok, nil
}

func (e *execution) executeAction(i int) error {
	span, end := vm.StartSpan(e, "ExecuteAction")
	defer end()
	span.SetAttributes(
		attribute.Int("Number", i),
		attribute.String("Postscript", e.Actions[i].String()))

	if i < 0 || i >= len(e.Actions) {
		return fmt.Errorf("action %d out of range", i)
	}
	err := vm.Framed{Value: e.Actions[i]}.Execute(e.State)
	if err != nil {
		return fmt.Errorf("action %d: %w", i, err)
	}
	return nil
}
