package vm

type Name interface {
	Value
	Name() string
}

type ExecutableName string
type LiteralName string

var _ Name = ExecutableName("")
var _ Name = LiteralName("")

func (n ExecutableName) Name() string              { return string(n) }
func (n ExecutableName) String() string            { return string(n) }
func (n ExecutableName) AsLiteral() (Value, error) { return LiteralName(n), nil }

func (n LiteralName) Name() string                 { return string(n) }
func (n LiteralName) String() string               { return "/" + string(n) }
func (n LiteralName) AsExecutable() (Value, error) { return ExecutableName(n), nil }

func (n ExecutableName) Execute(s State) error {
	v, err := Resolve(s, n)
	if err != nil {
		return err
	}
	u, err := v.Load(s)
	if err != nil {
		return err
	}
	return Execute(s, u)
}

type CompoundName struct {
	Entity     Name
	Member     Name
	Executable bool
}

func (n CompoundName) Name() string { return n.Entity.Name() + "." + n.Member.Name() }

func (n CompoundName) String() string {
	if n.Executable {
		return n.Name()
	}
	return "/" + n.Name()
}

func (n CompoundName) AsLiteral() (Value, error) {
	if !n.Executable {
		return n, nil
	}
	m := n
	m.Executable = false
	return m, nil
}

func (n CompoundName) AsExecutable() (Value, error) {
	if !n.Executable {
		return n, nil
	}
	m := n
	m.Executable = true
	return m, nil
}

func (n CompoundName) Execute(s State) error {
	if !n.Executable {
		return s.Data().Push(n)
	}

	v, err := Resolve(s, n)
	if err != nil {
		return err
	}
	u, err := v.Load(s)
	if err != nil {
		return err
	}
	return Execute(s, u)
}
