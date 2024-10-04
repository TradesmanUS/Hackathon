package postscript

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

const (
	eof = rune(-1)
	nul = rune(0)
	bom = rune(0xFEFF)
)

// Mode is the mode of the scanner
type Mode uint

const (
	// ScanDefault ignores comments and whitespace
	ScanDefault Mode = 0
	// ScanComments includes comment tokens
	ScanComments Mode = 1 << iota
	// ScanWhitespace includes whitespace tokens
	ScanWhitespace
)

// IsSet returns whether mode N is set on mode M
func (m Mode) IsSet(n Mode) bool { return m&n != 0 }

// IsUnset returns whether mode N is not set on mode M
func (m Mode) IsUnset(n Mode) bool { return m&n == 0 }

// An ErrorHandler handles error messages
type ErrorHandler func(pos int, msg string)

// A Scanner scans tokens from source text.
type Scanner struct {
	src  []byte
	mode Mode
	err  ErrorHandler

	char        rune
	offset, pos int
	// insertSemi  bool

	ErrorCount int
}

// Init initializes the scanner
func (s *Scanner) Init(src []byte, mode Mode, err ErrorHandler) {
	s.src = src
	s.mode = mode
	s.err = err

	s.char = nul
	s.offset = 0
	s.pos = 0
	// s.insertSemi = false
	s.ErrorCount = 0

	s.next()
	if s.char == bom {
		s.next() // ignore BOM at src beginning
	}
}

// Scan reads the next token from the source text
func (s *Scanner) Scan() (pos int, tok Token, lit string) {
scan:
	pos = s.offset // current token start

	var r = s.char
	switch {
	case isWhitespace(r):
		ws := s.scanWhitespace()
		if ScanWhitespace.IsUnset(s.mode) {
			goto scan
		}
		lit = ws
		tok = WHITESPACE
		return

	case isLetter(r):
		tok = SYMBOL
		lit = s.scanSymbol()
		return

	case '0' <= r && r <= '9':
		tok, lit = s.scanNumber(false)
		return

	case r == '"':
		tok = STRING
		lit = s.scanQuotedString(r)
		return

	case r == '`':
		tok = RAW_STRING
		lit = s.scanRawString()
		return

	case r == eof:
		tok = EOF
		return
	}

	s.next()
	switch {
	case r == ',':
		tok = COMMA
		lit = ","

	case r == '+':
		tok = ADD
		lit = "+"

	case r == '*':
		tok = MUL
		lit = "*"

	case r == '%':
		tok = MOD
		lit = "%"

	case r == '^':
		tok = POW
		lit = "^"

	case r == '¬':
		tok = NOT
		lit = "¬"

	case r == '∧':
		tok = AND
		lit = "∧"

	case r == '∨':
		tok = OR
		lit = "∨"

	case r == '(':
		tok = LPAREN
		lit = "("

	case r == '[':
		tok = LBRACK
		lit = "["

	case r == '{':
		tok = LBRACE
		lit = "{"

	case r == ')':
		tok = RPAREN
		lit = ")"

	case r == ']':
		tok = RBRACK
		lit = "]"

	case r == '}':
		tok = RBRACE
		lit = "}"

	case r == '.':
		if isDigit(s.char) {
			tok, lit = s.scanNumber(true)
		} else {
			tok = DOT
			lit = "."
		}

	case r == '/':
		if isLetter(s.char) {
			tok = SYMBOL
			lit = "/" + s.scanSymbol()
		} else {
			tok = DIV
			lit = "/"
		}

	case r == '|':
		if s.char == '|' {
			s.next()
			tok = OR
			lit = "||"
		} else {
			tok = ILLEGAL
			lit = string(r)
		}

	case r == '&':
		if s.char == '&' {
			s.next()
			tok = AND
			lit = "&&"
		} else {
			tok = ILLEGAL
			lit = string(r)
		}

	case r == '-':
		// TODO What about -.5?
		if isDigit(s.char) {
			tok, lit = s.scanNumber(false)
			lit = "-" + lit
		} else {
			tok = SUB
			lit = "-"
		}

	case r == '!':
		if s.char == '=' {
			s.next()
			tok = NEQ
			lit = "!="
		} else {
			tok = NOT
			lit = "!"
		}

	case r == '<':
		if s.char == '=' {
			s.next()
			tok = LEQ
			lit = "<="
		} else {
			tok = LSS
			lit = "<"
		}

	case r == '>':
		if s.char == '=' {
			s.next()
			tok = GEQ
			lit = ">="
		} else {
			tok = GTR
			lit = ">"
		}

	case r == '=':
		if s.char == '=' {
			s.next()
			tok = EQU
			lit = "=="
		} else if s.char == '<' {
			s.next()
			tok = LEQ
			lit = "=<"
		} else if s.char == '>' {
			s.next()
			tok = GEQ
			lit = "=>"
		} else {
			tok = ILLEGAL
			lit = string(r)
		}

	default:
		tok = ILLEGAL
		lit = string(r)
		s.error(pos, fmt.Sprintf("illegal token %q", lit))
	}
	return
}

func (s *Scanner) next() {
	if s.pos >= len(s.src) {
		s.offset = len(s.src)
		s.char = eof
		return
	}

	s.offset = s.pos

	var r, w = rune(s.src[s.pos]), 1
	switch {
	case r == nul:
		s.error(s.offset, "illegal character NUL")

	case r >= utf8.RuneSelf: // non-ASCII
		r, w = utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && w == 1 {
			s.error(s.offset, "illegal UTF-8 encoding")
		} else if r == bom && s.offset > 0 {
			s.error(s.offset, "illegal byte order mark")
		}
	}
	s.pos += w
	s.char = r
}

func (s *Scanner) peek() rune {
	if s.pos >= len(s.src) {
		return eof
	}

	var r = rune(s.src[s.pos])
	if r >= utf8.RuneSelf { // non-ASCII
		r, _ = utf8.DecodeRune(s.src[s.pos:])
	}

	return r
}

func (s *Scanner) error(offset int, msg string) {
	if s.err != nil {
		s.err(offset, msg)
	}
	s.ErrorCount++
}

func (s *Scanner) scanComment(r rune) string {
	// initial rune already consumed; current rune is '*' or '-'
	var offset = s.offset - 1
	var hasCR = false

	if s.char == r {
		// line comment ('--')
		s.next()
		for s.char != '\n' && s.char != eof {
			if s.char == '\r' {
				hasCR = true
			}
			s.next()
		}
		goto done
	}

	/* block comment */
	s.next()
	for s.char != eof {
		var r = s.char
		if r == '\r' {
			hasCR = true
		}
		s.next()
		if r == '*' && s.char == ')' {
			s.next()
			goto done
		}
	}

	s.error(offset, "unterminated comment")

done:
	var lit = s.src[offset:s.offset]
	if hasCR {
		lit = stripCR(lit)
	}
	return string(lit)
}

func (s *Scanner) scanNumber(hasPoint bool) (Token, string) {
	var start = s.offset
	var tok = INTEGER

	if hasPoint {
		start--
		tok = REAL
		s.scanDigits(10)
		goto exponent
	}

	if s.char == '0' {
		var start = s.offset
		s.next()

		if s.char == 'x' || s.char == 'X' {
			s.next()
			s.scanDigits(16)
			if s.offset-start <= 2 {
				// only scanned "0x" or "0X"
				s.error(start, "illegal hexadecimal number")
			}

		} else {
			decimal := false
			s.scanDigits(8)

			if s.char == '8' || s.char == '9' {
				decimal = true
				s.scanDigits(10)
			}

			if s.char == '.' || s.char == 'e' || s.char == 'E' {
				goto fraction
			}

			if decimal {
				s.error(start, "illegal octal number")
			}
		}

		goto exit
	}

	s.scanDigits(10)

fraction:
	if s.char == '.' {
		tok = REAL
		s.next()
		s.scanDigits(10)
	}

exponent:
	if s.char == 'e' || s.char == 'E' {
		tok = REAL
		s.next()

		if s.char == '-' || s.char == '+' {
			s.next()
		}

		if digitVal(s.char) < 10 {
			s.scanDigits(10)
		} else {
			s.error(start, "illegal floating-point exponent")
		}
	}

exit:
	return tok, string(s.src[start:s.offset])
}

func (s *Scanner) scanDigits(base int) {
	for digitVal(s.char) < base {
		s.next()
	}
}

func (s *Scanner) scanEscape() bool {
	var start = s.offset

	var n int
	var base, max uint32
	switch s.char {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '[', ']', '"', '\\', '\'':
		s.next()
		return true

	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255

	case 'x':
		s.next()
		n, base, max = 2, 16, 255

	case 'u':
		s.next()
		n, base, max = 4, 16, unicode.MaxRune

	case 'U':
		s.next()
		n, base, max = 8, 16, unicode.MaxRune

	case eof:
		s.error(start, "unterminated escape sequence")
		return false

	default:
		s.error(start, "unknown escape sequence")
		return false
	}

	var x uint32
	for n > 0 {
		var d = uint32(digitVal(s.char))
		if d < base {
			x = x*base + d
			s.next()
			n--
			continue
		}

		if s.char == eof {
			s.error(s.offset, "unterminated escape sequence")
		} else {
			s.error(s.offset, fmt.Sprintf("illegal character %#U in escape sequence", s.char))
		}
		return false
	}

	if x > max || 0xD800 <= x && x < 0xE000 {
		s.error(start, "escape sequence is invalid Unicode code point")
		return false
	}

	return true
}

func (s *Scanner) scanWhitespace() string {
	var start = s.offset
	for isWhitespace(s.char) {
		s.next()
	}
	return string(s.src[start:s.offset])
}

func (s *Scanner) scanSymbol() string {
	var start = s.offset
	for !isWhitespace(s.char) && s.char != eof {
		s.next()
	}
	return string(s.src[start:s.offset])
}

func (s *Scanner) scanRawString() string {
	var start = s.offset
	var hasDG = false

scan:
	for {
		s.next()

		switch s.char {
		case eof:
			s.error(start, "unterminated raw string")
			break scan

		case '`':
			s.next()
			if s.char != '`' {
				break scan
			} else {
				hasDG = true
			}
		}
	}

	var lit = s.src[start:s.offset]
	if hasDG {
		lit = collapseDoubleGraves(lit)
	}
	return string(lit)
}

func (s *Scanner) scanQuotedString(quote rune) string {
	var start = s.offset

	s.next()

scan:
	for {
		switch s.char {
		case eof:
			s.error(start, "unterminated string")
			break scan

		case quote:
			s.next()
			break scan

		case '\\':
			s.next()
			if s.char == '\n' {
				s.next()
			} else if s.char == '\r' {
				s.next()
				if s.char == '\n' {
					s.next()
				}
			} else {
				s.scanEscape()
			}

		default:
			s.next()
		}
	}

	var lit = s.src[start:s.offset]
	return string(lit)
}
