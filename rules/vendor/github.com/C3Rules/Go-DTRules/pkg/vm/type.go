package vm

type Type int

const (
	NullType Type = iota
	NameType
	NumberType
	BooleanType
	DateTimeType
	StringType
	ArrayType
	FunctionType

	ExternalType Type = 1 << 10
)

func (null) Type() Type             { return NullType }
func (Number[T]) Type() Type        { return NumberType }
func (boolean) Type() Type          { return BooleanType }
func (datetime) Type() Type         { return DateTimeType }
func (ExecutableArray) Type() Type  { return ArrayType }
func (LiteralArray) Type() Type     { return ArrayType }
func (ExecutableName) Type() Type   { return NameType }
func (LiteralName) Type() Type      { return NameType }
func (CompoundName) Type() Type     { return NameType }
func (ExecutableString) Type() Type { return StringType }
func (LiteralString) Type() Type    { return StringType }
func (Function) Type() Type         { return FunctionType }
