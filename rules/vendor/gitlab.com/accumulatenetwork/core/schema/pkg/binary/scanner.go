package binary

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// An scanState is a state of [scanner].
type scanState int

// An scanAction is an action for [scanner.scan].
type scanAction int

const (
	// scanAtInit is the state of the [scanner] after initialization, before
	// [scanner.scan] is called.
	scanAtInit scanState = iota
	// scanInObject indicates an object is open.
	scanInObject
	// scanInField indicates a field is open.
	scanInField
	// scanAtEnd indicates the scanner has scanted a complete value.
	scanAtEnd

	// scanVarWidth scans a variable-width value.
	scanVarWidth scanAction = iota
	// scanFixedWidth scans a fixed-width value.
	scanFixedWidth
	// scanField scans an object field number.
	scanField
	// scanNoField acts as if a field number is scanned without actually reading
	// a field number.
	scanNoField
	// scanStartObject scans the start of an object.
	scanStartObject
	// scanStartNestedObject scans the start of a nested object.
	scanStartNestedObject
	// scanEndObject scans the end of an object.
	scanEndObject
)

// scanner scans a binary value.
type scanner struct {
	common
	in    limitReader
	state []scanState
}

func (s *scanner) current() scanState {
	// If the state is empty, we're at the end of the stream
	if len(s.state) == 0 {
		return scanAtEnd
	}
	return s.state[len(s.state)-1]
}

// scan updates the scanner state according to the action that was executed.
func (s *scanner) scan(act scanAction) (uint64, error) {
	state, ok := s.allow(act)
	if !ok {
		return 0, fmt.Errorf("invalid call to didscan: %w", act.error(state))
	}

	// Is this a nested object?
	var nested bool
	switch act {
	case scanStartObject:
		nested = state != scanAtInit
	case scanEndObject:
		nested = len(s.state) > 1
	}

	// Read the value
	var val uint64
	var err error
	switch {
	case act == scanField:
		b, err := s.in.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == EmptyObject {
			return 0, io.EOF
		}
		if b < 1 || b > MaxFieldID {
			return 0, ErrInvalidFieldNumber
		}

		val = uint64(b)

	case act == scanVarWidth:
		val, err = binary.ReadUvarint(&s.in)

	case nested && act == scanStartObject:
		val, err = binary.ReadUvarint(&s.in)
	}
	if err != nil {
		return 0, err
	}

	// Discard extra data
	if !s.leaveTrailing && act == scanEndObject && s.in.Len() != 0 {
		_, err = io.Copy(io.Discard, &s.in)
		if err != nil {
			return 0, err
		}
	}

	// Manage reader limits for a nested object
	if nested {
		switch act {
		case scanStartObject:
			s.in.push(val)
		case scanEndObject:
			s.in.pop()
		}
	}

	// Track the field number
	switch act {
	case scanStartObject:
		s.pushScope()

	case scanField:
		if !s.ignoreFieldOrder && uint(val) < s.lastField[len(s.lastField)-1] {
			return 0, ErrFieldsOutOfOrder
		}
		if val != EmptyObject {
			s.lastField[len(s.lastField)-1] = uint(val)
		} else {
			err = io.EOF
		}

	case scanEndObject:
		s.popScope()
	}

	// Pop marker states
	switch state {
	case scanAtInit, scanInField:
		s.pop()
	}
	switch act {
	case scanStartObject:
		goto push
	case scanEndObject:
		s.pop()
	}

	// Check if we're done
	if len(s.state) == 0 {
		return val, err
	}

	// Push marker states
push:
	switch act {
	case scanField, scanNoField:
		if val != EmptyObject {
			s.push(scanInField)
		}
	case scanStartObject:
		s.push(scanInObject)
	}

	return val, err
}

// allow verifies that the specified action can be executed given the current
// state.
func (s *scanner) allow(a scanAction) (scanState, bool) {
	switch s.current() {
	case scanAtInit:
		// A value or start of object can be scanted at init
		switch a {
		case scanVarWidth, scanFixedWidth, scanStartObject:
			return scanAtInit, true
		}
		return scanAtInit, false

	case scanInObject:
		// A field number or end of object can be scanted when within an object
		// and no field number has been written
		switch a {
		case scanField, scanNoField, scanEndObject:
			return scanInObject, true
		}
		return scanInObject, false

	case scanInField:
		// A value or start of object can be scanted when within an
		// object and a field name has been written
		switch a {
		case scanVarWidth, scanFixedWidth, scanStartObject:
			return scanInField, true
		}
		return scanInField, false

	case scanAtEnd:
		return scanAtEnd, false
	}

	panic("invalid state")
}

func (s *scanner) push(state scanState) {
	s.state = append(s.state, state)
}

func (s *scanner) pop() {
	s.state = s.state[:len(s.state)-1]
}

func (a scanAction) error(s scanState) error {
	var err error
	switch s {
	case scanAtInit:
		err = errors.New("expecting value (at init)")
	case scanInObject:
		err = errors.New("expecting field (in object)")
	case scanInField:
		err = errors.New("expecting value (in field)")
	case scanAtEnd:
		err = errors.New("end of stream")
	}

	switch a {
	case scanStartObject:
		return fmt.Errorf("cannot start object: %w", err)
	case scanEndObject:
		return fmt.Errorf("cannot end object: %w", err)
	case scanField, scanNoField:
		return fmt.Errorf("cannot scan field name: %w", err)
	case scanVarWidth, scanFixedWidth:
		return fmt.Errorf("cannot scan value: %w", err)
	}
	panic("invalid action")
}
