package binary

import (
	"bytes"
)

// Unmarshal decodes a binary-encoded value.
func Unmarshal(b []byte, v any) error {
	buf := bytes.NewReader(b)
	dec := NewDecoder(buf)
	return dec.Decode(v)
}
