package binary

import (
	"bytes"
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
	// emitInObject indicates an object is open.
	emitInObject
	// emitInField indicates a field is open.
	emitInField
	// emitAtEnd indicates the emitter has emitted a complete value.
	emitAtEnd

	// emitValue emits a value.
	emitValue emitAction = iota
	// emitField emits an object field number.
	emitField
	// emitNoField acts as if a field number is emitted without actually reading
	// a field number.
	emitNoField
	// emitStartObject emits the start of an object.
	emitStartObject
	// emitEndObject emits the end of an object.
	emitEndObject
)

// emitter emits a binary value.
type emitter struct {
	common
	outs  []LenWriter
	state []emitState
}

func (e *emitter) current() emitState {
	// If the state is empty, we're at the end of the stream
	if len(e.state) == 0 {
		return emitAtEnd
	}
	return e.state[len(e.state)-1]
}

func (e *emitter) out() LenWriter {
	return e.outs[len(e.outs)-1]
}

// emit calls canEmit and didEmit. This is useful for actions that do not
// require writing data.
func (e *emitter) emit(a emitAction) error {
	s, ok := e.canEmit(a)
	if !ok {
		return a.error(s)
	}

	switch a {
	case emitStartObject:
		e.pushScope()

		// If the object is nested
		if s == emitAtInit {
			break
		}

		// Push an intermediate buffer
		buf := e.bufferPool.Get()
		e.outs = append(e.outs, buf)

	case emitEndObject:
		e.popScope()

		// If the object is empty, write an empty object marker
		if e.out().Len() == 0 {
			_, err := e.out().Write([]byte{EmptyObject})
			if err != nil {
				return err
			}
		}

		// If the object is nested
		if len(e.outs) == 1 {
			break
		}

		// Pop the intermediate buffer
		buf := e.out().(*bytes.Buffer)
		i := len(e.outs) - 1
		e.outs[i] = nil
		e.outs = e.outs[:i]

		// Write the length
		err := writeUint(e.out(), uint64(buf.Len()))
		if err != nil {
			return err
		}

		// Copy the data
		_, err = io.Copy(e.out(), buf)
		if err != nil {
			return err
		}

		// Put the buffer back in the pool
		e.bufferPool.Put(buf)
	}

	// Pop marker states
	switch s {
	case emitAtInit, emitInField:
		e.pop()
	}
	switch a {
	case emitStartObject:
		goto push
	case emitEndObject:
		e.pop()
	}

	// Check if we're done
	if len(e.state) == 0 {
		return nil
	}

	// Push marker states
push:
	switch a {
	case emitField:
		e.push(emitInField)
	case emitStartObject:
		e.push(emitInObject)
	}

	return nil
}

// canEmit verifies that the specified action can be executed given the current
// state.
func (e *emitter) canEmit(a emitAction) (emitState, bool) {
	switch e.current() {
	case emitAtInit:
		// A value or start of object can be emitted at init
		switch a {
		case emitValue, emitStartObject:
			return emitAtInit, true
		}
		return emitAtInit, false

	case emitInObject:
		// A field number or end of object can be emitted when within an object
		// and no field number has been written
		switch a {
		case emitField, emitEndObject:
			return emitInObject, true
		}
		return emitInObject, false

	case emitInField:
		// A value or start of object can be emitted when within an
		// object and a field name has been written
		switch a {
		case emitValue, emitStartObject:
			return emitInField, true
		}
		return emitInField, false

	case emitAtEnd:
		return emitAtEnd, false
	}

	panic("invalid state")
}

func (e *emitter) push(s emitState) {
	e.state = append(e.state, s)
}

func (e *emitter) pop() {
	e.state = e.state[:len(e.state)-1]
}

func (a emitAction) error(s emitState) error {
	var err error
	switch s {
	case emitAtInit:
		err = errors.New("expecting value (at init)")
	case emitInObject:
		err = errors.New("expecting field (in object)")
	case emitInField:
		err = errors.New("expecting value (in field)")
	case emitAtEnd:
		err = errors.New("end of stream")
	}

	switch a {
	case emitStartObject:
		return fmt.Errorf("cannot start object: %w", err)
	case emitEndObject:
		return fmt.Errorf("cannot end object: %w", err)
	case emitField:
		return fmt.Errorf("cannot emit field id: %w", err)
	case emitValue:
		return fmt.Errorf("cannot emit value: %w", err)
	}
	panic("invalid action")
}
