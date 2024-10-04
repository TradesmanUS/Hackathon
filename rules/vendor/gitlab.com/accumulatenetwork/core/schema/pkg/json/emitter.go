package json

import (
	"errors"
	"fmt"
	"io"
)

// An emitState is a state of [emitter].
type emitState int

// An emitAction is an action for [emitter.emit].
type emitAction int

const (
	// emitAtInit is the state of the [emitter] after initialization, before
	// [emitter.emit] is called.
	emitAtInit emitState = iota
	// emitInArray indicates an array is open.
	emitInArray
	// emitInObject indicates an object is open.
	emitInObject
	// emitInField indicates a field is open.
	emitInField
	// emitNeedComma indicates a comma is needed if another value is written.
	emitNeedComma
	// emitAtEnd indicates the emitter has emitted a complete value.
	emitAtEnd

	// emitValue emits a value.
	emitValue emitAction = iota
	// emitField emits an object field name.
	emitField
	// emitStartArray emits the start of an array.
	emitStartArray
	// emitStartObject emits the start of an object.
	emitStartObject
	// emitEndArray emits the end of an array.
	emitEndArray
	// emitEndObject emits the end of an object.
	emitEndObject
)

// emitter emits a JSON value.
type emitter struct {
	out   io.Writer
	state []emitState
}

// emit emits a token as specified by the action.
func (e *emitter) emit(act emitAction, encode func(any) error, value any) error {
	// Is the action allowed?
	s0, s1, ok := e.allowAction(act)
	if !ok {
		return act.error(s0)
	}

	// Write the comma (unless we're ending the array or object)
	if s1 == emitNeedComma && act != emitEndArray && act != emitEndObject {
		_, err := e.out.Write([]byte{','})
		if err != nil {
			return err
		}
	}

	// Write the value or field name
	switch act {
	case emitValue, emitField:
		err := encode(value)
		if err != nil {
			return err
		}
	}

	// Write the delimiter
	var err error
	switch act {
	case emitField:
		_, err = e.out.Write([]byte{':'})
	case emitStartArray:
		_, err = e.out.Write([]byte{'['})
	case emitStartObject:
		_, err = e.out.Write([]byte{'{'})
	case emitEndArray:
		_, err = e.out.Write([]byte{']'})
	case emitEndObject:
		_, err = e.out.Write([]byte{'}'})
	}
	if err != nil {
		return err
	}

	// Pop marker states
	switch s1 {
	case emitNeedComma:
		e.pop()
	}
	switch s0 {
	case emitAtInit, emitInField:
		e.pop()
	}
	switch act {
	case emitStartObject, emitStartArray:
		goto push
	case emitEndArray, emitEndObject:
		e.pop()
	}

	// Check if we're done
	if len(e.state) == 0 {
		return nil
	}

	// Push marker states
push:
	switch act {
	case emitValue,
		emitEndArray,
		emitEndObject:
		e.push(emitNeedComma)
	case emitField:
		e.push(emitInField)
	case emitStartArray:
		e.push(emitInArray)
	case emitStartObject:
		e.push(emitInObject)
	}

	return nil
}

// allowAction verifies that the specified action can be executed given the
// current state.
func (e *emitter) allowAction(a emitAction) (s0, s1 emitState, _ bool) {
	// If the state is empty, we're at the end of the stream
	if len(e.state) == 0 {
		return emitAtEnd, s1, false
	}

	s0 = e.state[len(e.state)-1]
afterComma:
	switch s0 {
	case emitAtInit:
		// A value or start of array or object can be emitted at init
		switch a {
		case emitValue, emitStartArray, emitStartObject:
			return emitAtInit, s1, true
		}
		return emitAtInit, s1, false

	case emitInArray:
		// A value, start of array or object, or end of array can be emitted
		// when within an array
		switch a {
		case emitValue, emitStartArray, emitStartObject, emitEndArray:
			return emitInArray, s1, true
		}
		return emitInArray, s1, false

	case emitInObject:
		// A field name or end of object can be emitted when within an object
		// and no field name has been written
		switch a {
		case emitField, emitEndObject:
			return emitInObject, s1, true
		}
		return emitInObject, s1, false

	case emitInField:
		// A value or start of array or object can be emitted when within an
		// object and a field name has been written
		switch a {
		case emitValue, emitStartArray, emitStartObject:
			return emitInField, s1, true
		}
		return emitInField, s1, false

	case emitNeedComma:
		// Process the previous state, which must be in-array or in-object
		if len(e.state) < 2 {
			break
		}
		s0, s1 = e.state[len(e.state)-2], emitNeedComma
		switch s0 {
		case emitInArray, emitInObject:
			goto afterComma
		}
	}

	panic("invalid state")
}

func (e *emitter) push(s ...emitState) {
	e.state = append(e.state, s...)
}

func (e *emitter) pop() {
	e.state = e.state[:len(e.state)-1]
}

func (a emitAction) error(s emitState) error {
	var err error
	switch s {
	case emitAtInit:
		err = errors.New("expecting value (at init)")
	case emitInArray:
		err = errors.New("expecting value (in array)")
	case emitInObject:
		err = errors.New("expecting field (in object)")
	case emitInField:
		err = errors.New("expecting value (in field)")
	case emitNeedComma:
		err = errors.New("expecting value (need comma)")
	case emitAtEnd:
		err = errors.New("end of stream")
	}

	switch a {
	case emitStartArray:
		return fmt.Errorf("cannot start array: %w", err)
	case emitEndArray:
		return fmt.Errorf("cannot end array: %w", err)
	case emitStartObject:
		return fmt.Errorf("cannot start object: %w", err)
	case emitEndObject:
		return fmt.Errorf("cannot end object: %w", err)
	case emitField:
		return fmt.Errorf("cannot emit field name: %w", err)
	case emitValue:
		return fmt.Errorf("cannot emit value: %w", err)
	}
	panic("invalid action")
}
