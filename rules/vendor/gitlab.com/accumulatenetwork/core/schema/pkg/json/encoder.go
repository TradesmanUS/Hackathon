package json

import (
	"errors"
	"io"
	"reflect"

	"gitlab.com/accumulatenetwork/core/schema/pkg/traverse"
)

type Encoder struct {
	emitter
	context *traverse.Context[reflect.Value]
}

func NewEncoder(w io.Writer) *Encoder {
	return NewEncoderWithContext(w, nil)
}

func NewEncoderWithContext(w io.Writer, ctx *traverse.Context[reflect.Value]) *Encoder {
	e := new(Encoder)
	e.state = []emitState{emitAtInit}
	e.emitter.out = w
	e.context = ctx
	return e
}

func (e *Encoder) Context() *traverse.Context[reflect.Value] {
	if e.context == nil {
		e.context = new(traverse.Context[reflect.Value])
	}
	return e.context
}

func (e *Encoder) Encode(v any) error      { return e.emit(emitValue, e.encode, v) }
func (e *Encoder) Field(name string) error { return e.emit(emitField, e.encode, name) }
func (e *Encoder) StartArray() error       { return e.emit(emitStartArray, nil, nil) }
func (e *Encoder) StartObject() error      { return e.emit(emitStartObject, nil, nil) }
func (e *Encoder) EndArray() error         { return e.emit(emitEndArray, nil, nil) }
func (e *Encoder) EndObject() error        { return e.emit(emitEndObject, nil, nil) }

func (e *Encoder) Done() error {
	if len(e.state) > 0 {
		return errors.New("not done")
	}
	return nil
}
