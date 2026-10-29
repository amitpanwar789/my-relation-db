// This file tests how SQL characters become tokens.
// It compares token kinds, values, original text, and source positions.
// It covers quotes, Unicode strings, whitespace, numbers, and invalid input.
// These tests run independently of the shell and database storage.

package sql_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name, input string
		kinds       []sql.TokenKind
		values      []string
	}{
		{"keywords and identifiers", "sElEcT _ID2, Name FROM UsErS;", []sql.TokenKind{sql.Select, sql.Identifier, sql.Comma, sql.Identifier, sql.From, sql.Identifier, sql.Semicolon, sql.EOF}, []string{"SELECT", "_id2", ",", "name", "FROM", "users", ";", ""}},
		{"punctuation", "(*,);", []sql.TokenKind{sql.LeftParen, sql.Star, sql.Comma, sql.RightParen, sql.Semicolon, sql.EOF}, []string{"(", "*", ",", ")", ";", ""}},
		{"literals", "-42 +09 0 TRUE false NULL", []sql.TokenKind{sql.Integer, sql.Integer, sql.Integer, sql.True, sql.False, sql.Null, sql.EOF}, []string{"-42", "+09", "0", "TRUE", "FALSE", "NULL", ""}},
		{"strings", "'' 'O''Brien;नमस्ते' 'a\nb' '\\x'", []sql.TokenKind{sql.String, sql.String, sql.String, sql.String, sql.EOF}, []string{"", "O'Brien;नमस्ते", "a\nb", "\\x", ""}},
		{"whitespace", " \t\r\n", []sql.TokenKind{sql.EOF}, []string{""}},
		{"keyword prefixes", "selectable from_here nullish", []sql.TokenKind{sql.Identifier, sql.Identifier, sql.Identifier, sql.EOF}, []string{"selectable", "from_here", "nullish", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := sql.Tokenize(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			var kinds []sql.TokenKind
			var values []string
			for _, token := range tokens {
				kinds = append(kinds, token.Kind)
				values = append(values, token.Value)
				if token.Kind != sql.EOF && tt.input[token.Pos.Offset:token.Pos.Offset+len(token.Lexeme)] != token.Lexeme {
					t.Fatalf("lexeme not at recorded offset: %+v", token)
				}
			}
			if !reflect.DeepEqual(kinds, tt.kinds) || !reflect.DeepEqual(values, tt.values) {
				t.Fatalf("got %v %q; want %v %q", kinds, values, tt.kinds, tt.values)
			}
		})
	}
}

func TestTokenPositions(t *testing.T) {
	tokens, err := sql.Tokenize(" \r\n'é'\tfoo\n;")
	if err != nil {
		t.Fatal(err)
	}
	want := []sql.Position{{Offset: 3, Line: 2, Column: 1}, {Offset: 8, Line: 2, Column: 5}, {Offset: 12, Line: 3, Column: 1}, {Offset: 13, Line: 3, Column: 2}}
	for i, pos := range want {
		if tokens[i].Pos != pos {
			t.Errorf("token %d position = %+v; want %+v", i, tokens[i].Pos, pos)
		}
	}
}

func TestTokenizeErrors(t *testing.T) {
	tests := []struct {
		input, message string
		line, column   int
	}{
		{"SELECT @", "unexpected character", 1, 8},
		{"\n'not closed", "unterminated string literal", 2, 1},
		{"'a''", "unterminated string literal", 1, 1},
		{"+", "expected digits", 1, 1},
		{"- 2", "expected digits", 1, 1},
		{"\xff", "invalid UTF-8", 1, 1},
		{"'é\xff'", "invalid UTF-8", 1, 3},
		{"名字", "unexpected character", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := sql.Tokenize(tt.input)
			var syntax *sql.SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("expected SyntaxError, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.message) || syntax.Pos.Line != tt.line || syntax.Pos.Column != tt.column {
				t.Fatalf("unexpected diagnostic: %v (%+v)", err, syntax.Pos)
			}
		})
	}
}
