package vm

import (
	"errors"
	"io"
	"strconv"
	"strings"

	ps "github.com/C3Rules/Go-DTRules/pkg/postscript"
)

type Scanner struct {
	ps  ps.Scanner
	err error

	EvaluateLiteralArrays bool
	Visit                 func(Value) Value
}

type ScannerError struct {
	Offset  int
	Message string
}

func (s *ScannerError) Error() string { return s.Message }

func (s *Scanner) Init(src []byte) {
	s.ps.Init(src, ps.ScanDefault, func(pos int, msg string) {
		s.err = &ScannerError{pos, msg}
	})
}

func (s *Scanner) Scan() (Value, error) {
	if s.err != nil {
		return nil, s.err
	}

	_, tok, lit := s.ps.Scan()
	if s.err != nil {
		return nil, s.err
	}
	return s.parse(tok, lit)
}

func (s *Scanner) parse(tok ps.Token, lit string) (v Value, err error) {
	if s.Visit != nil {
		defer func() {
			if err == nil {
				v = s.Visit(v)
			}
		}()
	}

	switch tok {
	case ps.EOF:
		return nil, io.EOF

	case ps.SYMBOL:
		if !strings.HasPrefix(lit, "/") {
			break
		}

		if i := strings.IndexByte(lit, '.'); i >= 0 {
			return CompoundName{
				Entity: LiteralName(lit[1:i]),
				Member: LiteralName(lit[i+1:]),
			}, nil
		}
		return LiteralName(lit[1:]), nil

	case ps.INTEGER:
		v, err := strconv.ParseInt(lit, 10, 64)
		return Number[int64]{v}, err

	case ps.REAL:
		v, err := strconv.ParseFloat(lit, 10)
		return Number[float64]{v}, err

	case ps.STRING:
		lit = strings.ReplaceAll(lit, "\n", `\n`)
		lit = strings.ReplaceAll(lit, "\r", `\r`)
		v, err := strconv.Unquote(lit)
		return LiteralString(v), err

	case ps.RAW_STRING:
		return LiteralString(lit), nil

	case ps.LBRACK:
		if !s.EvaluateLiteralArrays {
			break
		}
		values, err := s.parseUntil(ps.RBRACK)
		if err != nil {
			return nil, err
		}
		return (*LiteralArray)(&values), nil

	case ps.LBRACE:
		values, err := s.parseUntil(ps.RBRACE)
		if err != nil {
			return nil, err
		}
		return (*ExecutableArray)(&values), nil
	}

	op, ok := resolveOperator(lit)
	if ok {
		return op, nil
	}
	if i := strings.IndexByte(lit, '.'); i >= 0 {
		return CompoundName{
			Entity:     LiteralName(lit[:i]),
			Member:     LiteralName(lit[i+1:]),
			Executable: true,
		}, nil
	}
	return ExecutableName(lit), nil
}

func (s *Scanner) parseUntil(end ps.Token) ([]Value, error) {
	var values []Value
	_, tok, lit := s.ps.Scan()
	for s.err == nil && tok != end {
		v, err := s.parse(tok, lit)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		values = append(values, v)
		_, tok, lit = s.ps.Scan()
	}
	return values, s.err
}
