package binary

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"io"
	"math"
)

// Marshal binary-encodes the value.
func Marshal(v any) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := NewEncoder(buf)
	err := enc.Encode(v)
	return buf.Bytes(), err
}

func writeBool(wr io.Writer, v bool) error {
	var x byte
	if v {
		x = 1
	}

	if bw, ok := wr.(io.ByteWriter); ok {
		return bw.WriteByte(x)
	}

	_, err := wr.Write([]byte{x})
	return err
}

func writeInt(wr io.Writer, v int64) error {
	// Copied from [binary.PutVarint]
	ux := uint64(v) << 1
	if v < 0 {
		ux = ^ux
	}

	return writeUint(wr, ux)
}

func writeUint(wr io.Writer, v uint64) error {
	bwr, ok := wr.(io.ByteWriter)
	if !ok {
		var b [10]byte
		n := binary.PutUvarint(b[:], v)
		_, err := wr.Write(b[:n])
		return err
	}

	// Minimum allocation write, copied from [binary.PutUvarint]
	i := 0
	for v >= 0x80 {
		err := bwr.WriteByte(byte(v) | 0x80)
		if err != nil {
			return err
		}
		v >>= 7
		i++
	}
	return bwr.WriteByte(byte(v))
}

func writeFloat(wr io.Writer, v float64) error {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(v))
	_, err := wr.Write(b[:])
	return err
}

func writeBytes(wr io.Writer, v []byte) error {
	err := writeUint(wr, uint64(len(v)))
	if err != nil {
		return err
	}

	_, err = wr.Write(v)
	return err
}

func writeString(wr io.Writer, v string) error {
	sw, ok := wr.(io.StringWriter)
	if !ok {
		return writeBytes(wr, []byte(v))
	}

	err := writeUint(wr, uint64(len(v)))
	if err != nil {
		return err
	}

	_, err = sw.WriteString(v)
	return err
}

func writeM1(wr io.Writer, v encoding.BinaryMarshaler) error {
	b, err := v.MarshalBinary()
	if err != nil {
		return err
	}

	return writeBytes(wr, b)
}
