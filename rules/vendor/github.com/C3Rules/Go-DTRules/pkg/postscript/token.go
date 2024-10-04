package postscript

import "strconv"

type Token int

const (
	// Special tokens
	ILLEGAL Token = iota
	EOF
	WHITESPACE

	// Literals
	SYMBOL     // symbol
	INTEGER    // 1
	REAL       // 1.0
	STRING     // 'string'
	RAW_STRING // `string`

	// Delimiters, etc
	COMMA  // ,
	LPAREN // (
	LBRACK // [
	LBRACE // {
	RPAREN // )
	RBRACK // ]
	RBRACE // }

	// Operators
	DOT // .
	AT  // @

	ADD // +
	SUB // -
	MUL // *
	DIV // /
	MOD // %
	POW // ^
	NOT // !

	AND // &&
	OR  // ||

	EQU // ==
	NEQ // !=
	LSS // <
	GTR // >
	LEQ // <=
	GEQ // >=

	TOKEN_COUNT
)

var names = [...]string{
	ILLEGAL:    "ILLEGAL",
	EOF:        "EOF",
	WHITESPACE: "WHITESPACE",
	SYMBOL:     "SYMBOL",
	INTEGER:    "INTEGER",
	REAL:       "REAL",
	STRING:     "STRING",
	RAW_STRING: "RAW_STRING",
	COMMA:      ",",
	DOT:        "⋅",
	AT:         "@",
	ADD:        "+",
	SUB:        "-",
	MUL:        "*",
	DIV:        "/",
	MOD:        "%",
	POW:        "^",
	NOT:        "!",
	AND:        "∧",
	OR:         "∨",
	EQU:        "==",
	NEQ:        "≠",
	LSS:        "<",
	GTR:        ">",
	LEQ:        "≤",
	GEQ:        "≥",
	LPAREN:     "(",
	LBRACK:     "[",
	LBRACE:     "{",
	RPAREN:     ")",
	RBRACK:     "]",
	RBRACE:     "}",
}

func (tok Token) String() string {
	var s = ""

	if 0 <= tok && tok < Token(len(names)) {
		s = names[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}

	return s
}

func (tok Token) IsOneOf(toks ...Token) bool {
	for _, t := range toks {
		if tok == t {
			return true
		}
	}

	return false
}
