package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Decoder struct {
	data   []byte
	dec    *json.Decoder
	token  json.Token
	err    error
	offset int64

	CaptureSkippedFields bool
	skippedFields        extraStack
	lastField            string
}

func NewDecoder(b []byte) *Decoder {
	d := new(Decoder)
	d.data = b
	d.dec = json.NewDecoder(bytes.NewReader(b))
	d.next()
	return d
}

func (d *Decoder) Decode(v any) error {
	if ok, err := d.decode(v); ok {
		return err
	}

	b := d.capture()
	if d.err != nil {
		return d.err
	}
	return json.Unmarshal(b, v)
}

func (d *Decoder) Peek() json.Token {
	return d.token
}

func (d *Decoder) StartArray() error {
	return d.nextIsDelim('[')
}

func (d *Decoder) EndArray() error {
	for d.More() {
		d.Skip()
	}
	return d.nextIsDelim(']')
}

func (d *Decoder) StartObject() error {
	if d.CaptureSkippedFields {
		d.skippedFields = append(d.skippedFields, nil)
	}
	return d.nextIsDelim('{')
}

func (d *Decoder) EndObject() error {
	for d.More() {
		d.Field()
		d.Skip()
	}

	err := d.nextIsDelim('}')
	if d.CaptureSkippedFields {
		i := len(d.skippedFields) - 1
		d.skippedFields[i] = nil
		d.skippedFields = d.skippedFields[:i]
	}
	return err
}

func (d *Decoder) Skipped() *Extra {
	return d.skippedFields.current()
}

func (d *Decoder) Field() (string, error) {
	s, _ := nextIs[string](d)
	if d.CaptureSkippedFields {
		d.lastField = s
	}
	return s, d.err
}

func (d *Decoder) More() bool {
	if d.err != nil {
		return false
	}
	switch d.token {
	case nil, // Does this make sense? Write some tests.
		json.Delim(']'),
		json.Delim('}'):
		return false
	}
	return true
}

func (d *Decoder) Skip() error {
	field := d.lastField
	b := d.capture()
	if d.CaptureSkippedFields && d.err == nil && field != "" {
		d.Skipped().add(field, b)
	}
	return d.err
}

func (d *Decoder) capture() []byte {
	start := d.offset
	switch d.data[start] {
	case ':', ',':
		start++
	}

	d.skip()
	if d.err != nil {
		return nil
	}

	return d.data[start:d.offset]
}

func (d *Decoder) skip() {
	if d.err != nil {
		return
	}

	var delim json.Delim
	switch token := d.next().(type) {
	case bool, float64, string, nil:
		goto done
	case json.Delim:
		delim = token
	default:
		panic(fmt.Errorf("unexpected token type %T", token))
	}
	switch delim {
	case '{':
		if d.CaptureSkippedFields {
			d.skippedFields = append(d.skippedFields, nil)
		}
		for d.More() {
			d.Field()
			d.Skip()
		}
		d.EndObject()
	case '[':
		for d.More() {
			d.Skip()
		}
		d.EndArray()
	default:
		panic(fmt.Errorf("unexpected delimiter %q", delim))
	}

done:
	if errors.Is(d.err, io.EOF) {
		d.err = nil
	}
}

func (d *Decoder) nextIsDelim(want json.Delim) error {
	delim, ok := nextIs[json.Delim](d)
	if ok && delim != want {
		d.err = fmt.Errorf("expected %v, got %v", want, delim)
	}
	return d.err
}

func nextIs[V any](d *Decoder) (V, bool) {
	token := d.next()
	if d.err != nil {
		var z V
		return z, false
	}
	v, ok := token.(V)
	if !ok {
		d.err = fmt.Errorf("expected %T, got %T", v, token)
		return v, false
	}
	return v, true
}

func (d *Decoder) next() json.Token {
	if d.err != nil {
		return nil
	}

	if d.offset >= int64(len(d.data)) {
		d.err = io.EOF
		return nil
	}

	token := d.token
	d.lastField = ""

	// Handle EOF
	d.offset = d.dec.InputOffset()
	if d.offset >= int64(len(d.data)) {
		return token
	}

	d.token, d.err = d.dec.Token()
	return token
}
