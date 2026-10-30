// This file defines the small pieces of SQL recognized by the tokenizer.
// Each token records its kind, original text, value, and source position.
// It also defines syntax errors with a line and column for useful messages.
// The tokenizer and parser share these definitions.

package sql

import "fmt"

// Position locates input using a byte offset and one-based line/rune column.
type Position struct {
	Offset int
	Line   int
	Column int
}

type TokenKind string

const (
	EOF          TokenKind = "end of input"
	Identifier   TokenKind = "identifier"
	Integer      TokenKind = "integer literal"
	String       TokenKind = "string literal"
	LeftParen    TokenKind = "("
	RightParen   TokenKind = ")"
	Comma        TokenKind = ","
	Star         TokenKind = "*"
	Semicolon    TokenKind = ";"
	Create       TokenKind = "CREATE"
	Table        TokenKind = "TABLE"
	Insert       TokenKind = "INSERT"
	Into         TokenKind = "INTO"
	Values       TokenKind = "VALUES"
	Select       TokenKind = "SELECT"
	From         TokenKind = "FROM"
	IntegerType  TokenKind = "INTEGER"
	TextType     TokenKind = "TEXT"
	BooleanType  TokenKind = "BOOLEAN"
	True         TokenKind = "TRUE"
	False        TokenKind = "FALSE"
	Null         TokenKind = "NULL"
	Where        TokenKind = "WHERE"
	Update       TokenKind = "UPDATE"
	Set          TokenKind = "SET"
	Delete       TokenKind = "DELETE"
	And          TokenKind = "AND"
	Or           TokenKind = "OR"
	Is           TokenKind = "IS"
	Not          TokenKind = "NOT"
	Equal        TokenKind = "="
	NotEqual     TokenKind = "<>"
	Less         TokenKind = "<"
	LessEqual    TokenKind = "<="
	Greater      TokenKind = ">"
	GreaterEqual TokenKind = ">="
)

type Token struct {
	Kind   TokenKind
	Lexeme string
	Value  string
	Pos    Position
}

// SyntaxError covers lexical and grammar errors, never catalog validation.
type SyntaxError struct {
	Pos     Position
	Message string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
}
