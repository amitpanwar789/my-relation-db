// This file turns SQL text into tokens by reading its characters in order.
// It recognizes keywords, names, punctuation, numbers, and quoted strings.
// It decodes doubled quotes and records where each token begins.
// Invalid characters and unfinished strings produce positioned errors.

package sql

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

var keywords = map[string]TokenKind{
	"CREATE": Create, "TABLE": Table, "INSERT": Insert, "INTO": Into,
	"VALUES": Values, "SELECT": Select, "FROM": From,
	"INTEGER": IntegerType, "TEXT": TextType, "BOOLEAN": BooleanType,
	"TRUE": True, "FALSE": False, "NULL": Null,
	"WHERE": Where, "UPDATE": Update, "SET": Set, "DELETE": Delete,
	"AND": And, "OR": Or, "IS": Is, "NOT": Not,
}

type lexer struct {
	input string
	pos   Position
}

// Tokenize preserves original lexemes and decodes strings/normalizes names.
func Tokenize(input string) ([]Token, error) {
	l := lexer{input: input, pos: Position{Line: 1, Column: 1}}
	var tokens []Token
	for {
		token, err := l.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		if token.Kind == EOF {
			return tokens, nil
		}
	}
}

func (l *lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.input[l.pos.Offset:])
	l.pos.Offset += size
	if r == '\n' {
		l.pos.Line++
		l.pos.Column = 1
	} else {
		l.pos.Column++
	}
	return r
}

func (l *lexer) next() (Token, error) {
	for l.pos.Offset < len(l.input) && strings.ContainsRune(" \t\r\n", rune(l.input[l.pos.Offset])) {
		l.advance()
	}
	start := l.pos
	if start.Offset == len(l.input) {
		return Token{Kind: EOF, Pos: start}, nil
	}
	c := l.input[start.Offset]
	if isNameStart(c) {
		l.advance()
		for l.pos.Offset < len(l.input) && (isNameStart(l.input[l.pos.Offset]) || isDigit(l.input[l.pos.Offset])) {
			l.advance()
		}
		word := l.input[start.Offset:l.pos.Offset]
		if kind, ok := keywords[strings.ToUpper(word)]; ok {
			return l.token(kind, start, strings.ToUpper(word)), nil
		}
		return l.token(Identifier, start, strings.ToLower(word)), nil
	}
	if isDigit(c) || c == '+' || c == '-' {
		l.advance()
		if !isDigit(c) && (l.pos.Offset == len(l.input) || !isDigit(l.input[l.pos.Offset])) {
			return Token{}, &SyntaxError{start, "expected digits immediately after integer sign"}
		}
		for l.pos.Offset < len(l.input) && isDigit(l.input[l.pos.Offset]) {
			l.advance()
		}
		return l.token(Integer, start, l.input[start.Offset:l.pos.Offset]), nil
	}
	if c == '\'' {
		return l.stringToken(start)
	}
	if strings.ContainsRune("=<>!", rune(c)) {
		l.advance()
		if l.pos.Offset < len(l.input) {
			next := l.input[l.pos.Offset]
			if next == '=' && c != '=' || c == '<' && next == '>' {
				l.advance()
			}
		}
		spelling := l.input[start.Offset:l.pos.Offset]
		if spelling == "!" {
			return Token{}, &SyntaxError{start, "expected = after !"}
		}
		kind := TokenKind(spelling)
		if spelling == "!=" {
			kind = NotEqual
		}
		return l.token(kind, start, string(kind)), nil
	}
	kind := TokenKind(string(c))
	switch kind {
	case LeftParen, RightParen, Comma, Star, Semicolon:
		l.advance()
		return l.token(kind, start, string(c)), nil
	}
	r, size := utf8.DecodeRuneInString(l.input[start.Offset:])
	if r == utf8.RuneError && size == 1 {
		return Token{}, &SyntaxError{start, "invalid UTF-8"}
	}
	return Token{}, &SyntaxError{start, fmt.Sprintf("unexpected character %q", r)}
}

func (l *lexer) stringToken(start Position) (Token, error) {
	l.advance() // Opening quote.
	var value strings.Builder
	for l.pos.Offset < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos.Offset:])
		if r == utf8.RuneError && size == 1 {
			return Token{}, &SyntaxError{l.pos, "invalid UTF-8"}
		}
		l.advance()
		if r != '\'' {
			value.WriteRune(r)
			continue
		}
		if l.pos.Offset < len(l.input) && l.input[l.pos.Offset] == '\'' {
			l.advance()
			value.WriteRune('\'')
			continue
		}
		return l.token(String, start, value.String()), nil
	}
	return Token{}, &SyntaxError{start, "unterminated string literal"}
}

func (l *lexer) token(kind TokenKind, start Position, value string) Token {
	return Token{Kind: kind, Lexeme: l.input[start.Offset:l.pos.Offset], Value: value, Pos: start}
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
