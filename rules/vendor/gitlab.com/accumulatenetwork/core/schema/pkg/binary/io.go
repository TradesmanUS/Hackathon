package binary

import (
	"bufio"
	"io"
)

type byteScanner interface {
	io.Reader
	io.ByteScanner
}

func asByteScanner(r io.Reader) byteScanner {
	if r, ok := r.(byteScanner); ok {
		return r
	}
	return bufio.NewReader(r)
}

type limitReader struct {
	rd    byteScanner
	limit []uint64
}

func (l *limitReader) push(n uint64) {
	l.limit = append(l.limit, n)
}

func (l *limitReader) pop() {
	i := len(l.limit) - 1
	l.limit = l.limit[:i]
}

func (l *limitReader) remaining() uint64 {
	return l.limit[len(l.limit)-1]
}

func (l *limitReader) Len() int {
	if len(l.limit) > 0 {
		return int(l.limit[len(l.limit)-1])
	}
	if rd, ok := l.rd.(interface{ Len() int }); ok {
		return rd.Len()
	}
	return -1
}

func (l *limitReader) ReadByte() (byte, error) {
	if len(l.limit) == 0 {
		return l.rd.ReadByte()
	}

	max := l.limit[len(l.limit)-1]
	if max == 0 {
		return 0, io.EOF
	}
	b, err := l.rd.ReadByte()
	if err != nil {
		return 0, err
	}
	l.didRead(1)
	return b, nil
}

func (l *limitReader) UnreadByte() error {
	if len(l.limit) == 0 {
		return l.rd.UnreadByte()
	}

	err := l.rd.UnreadByte()
	if err != nil {
		return err
	}
	l.didRead(-1)
	return nil
}

func (l *limitReader) Read(b []byte) (int, error) {
	if len(l.limit) == 0 {
		return l.rd.Read(b)
	}

	max := l.limit[len(l.limit)-1]
	if len(b) > int(max) {
		b = b[:max]
	}
	n, err := l.rd.Read(b)
	l.didRead(n)
	return n, err
}

func (l *limitReader) didRead(n int) {
	for i := range l.limit {
		l.limit[i] -= uint64(n)
	}
}

// LenWriter is an [io.Writer] with a Len method.
type LenWriter interface {
	io.Writer
	Len() int
}

func asLenWriter(w io.Writer) LenWriter {
	if w, ok := w.(LenWriter); ok {
		return w
	}
	return &writer{Writer: w}
}

type writer struct {
	io.Writer
	len int
}

func (w *writer) Len() int {
	return w.len
}

func (w *writer) Write(b []byte) (int, error) {
	n, err := w.Writer.Write(b)
	if err != nil {
		return 0, err
	}
	w.len += n
	return n, err
}
