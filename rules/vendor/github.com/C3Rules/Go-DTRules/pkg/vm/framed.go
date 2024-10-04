package vm

type Framed struct {
	Value
}

func (f Framed) Execute(s State) error {
	_, err := ExecuteFramed(s, f.Value)
	return err
}

func ExecuteFramed(s State, v ...Value) ([]Value, error) {
	if s == nil {
		s = New()
	}
	err := PushDataFrame(s)
	if err != nil {
		return nil, err
	}
	err = PushEntityFrame(s)
	if err != nil {
		return nil, err
	}
	err = Execute(s, v...)
	if err != nil {
		return nil, err
	}
	_, err = PopEntityFrame(s)
	if err != nil {
		return nil, err
	}
	return PopDataFrame(s)
}

func ExecuteFramedString(s State, src string) ([]Value, error) {
	return ExecuteFramed(s, ExecutableString(src))
}
