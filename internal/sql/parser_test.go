// This file tests the structure and errors produced when parsing SQL.
// It compares valid commands with their expected statement descriptions.
// It checks malformed syntax and confirms semantic checks remain deferred.
// A fuzz test also tries generated input to find panics or invalid results.

package sql_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

func TestParseStatements(t *testing.T) {
	tests := []struct {
		input string
		want  sql.Statement
	}{
		{"CREATE TABLE Users (ID INTEGER, name TEXT, active BOOLEAN);", &sql.CreateTableStatement{Table: "users", Columns: []sql.ColumnDefinition{{Name: "id", Type: sql.TypeInteger}, {Name: "name", Type: sql.TypeText}, {Name: "active", Type: sql.TypeBoolean}}}},
		{"insert into Users (id, name, active, missing, other) values (-42, 'O''Brien', true, null, false);", &sql.InsertStatement{Table: "users", Columns: []string{"id", "name", "active", "missing", "other"}, Values: []sql.Literal{{Kind: sql.LiteralInteger, Value: "-42"}, {Kind: sql.LiteralText, Value: "O'Brien"}, {Kind: sql.LiteralBoolean, Value: "TRUE"}, {Kind: sql.LiteralNull}, {Kind: sql.LiteralBoolean, Value: "FALSE"}}}},
		{"SELECT * FROM unknown", &sql.SelectStatement{Table: "unknown", All: true}},
		{"\nSelect Name, ID\nFROM Users;\n", &sql.SelectStatement{Table: "users", Columns: []string{"name", "id"}}},
		{"INSERT INTO t (a, b, c) VALUES (+0001, '', 'नमस्ते');", &sql.InsertStatement{Table: "t", Columns: []string{"a", "b", "c"}, Values: []sql.Literal{{Kind: sql.LiteralInteger, Value: "+0001"}, {Kind: sql.LiteralText}, {Kind: sql.LiteralText, Value: "नमस्ते"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := sql.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) || got.Kind() != tt.want.Kind() {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseDefersSemanticValidation(t *testing.T) {
	for _, input := range []string{
		"SELECT missing FROM nonexistent;",
		"CREATE TABLE t (id INTEGER, id TEXT);",
		"INSERT INTO t (a, a) VALUES ('text');",
		"INSERT INTO t (a) VALUES (999999999999999999999999999999999999999);",
	} {
		if _, err := sql.Parse(input); err != nil {
			t.Errorf("semantic case rejected by parser: %s: %v", input, err)
		}
	}
}

func TestParseRejectsInvalidGrammar(t *testing.T) {
	inputs := []string{
		"", ";", "CREATE", "CREATE users (id INTEGER)", "CREATE TABLE",
		"CREATE TABLE t ()", "CREATE TABLE t (id)", "CREATE TABLE t (id FLOAT)",
		"CREATE TABLE t (id INTEGER,)", "CREATE TABLE t (id INTEGER", "CREATE TABLE t id INTEGER)",
		"INSERT", "INSERT t (a) VALUES (1)", "INSERT INTO", "INSERT INTO t VALUES (1)",
		"INSERT INTO t () VALUES (1)", "INSERT INTO t (a,) VALUES (1)",
		"INSERT INTO t (a) (1)", "INSERT INTO t (a) VALUES 1",
		"INSERT INTO t (a) VALUES ()", "INSERT INTO t (a) VALUES (1,)",
		"INSERT INTO t (a) VALUES (1", "INSERT INTO t (a) VALUES (name)",
		"INSERT INTO t (a) VALUES (1), (2)", "INSERT INTO t (a) VALUES (1.5)",
		"SELECT", "SELECT FROM t", "SELECT id, FROM t", "SELECT *, id FROM t",
		"SELECT id t", "SELECT id FROM", "SELECT id FROM t WHERE id = 1",
		"SELECT id FROM t junk", "SELECT id FROM t; SELECT * FROM t;",
		"SELECT id FROM t;;", "SELECT \"id\" FROM t", "UPDATE t SET a = 1;",
		"DELETE FROM t;", "CREATE INDEX i ON t (a);", "SELECT * FROM t -- comment",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			stmt, err := sql.Parse(input)
			var syntax *sql.SyntaxError
			if stmt != nil || !errors.As(err, &syntax) {
				t.Fatalf("expected nil statement and SyntaxError, got %#v, %v", stmt, err)
			}
		})
	}
}

func TestParseErrorLocation(t *testing.T) {
	_, err := sql.Parse("SELECT id,\n FROM users;")
	var syntax *sql.SyntaxError
	if !errors.As(err, &syntax) || syntax.Pos.Line != 2 || syntax.Pos.Column != 2 || !strings.Contains(err.Error(), "expected identifier, found \"FROM\"") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"SELECT * FROM users;", "CREATE TABLE t (a INTEGER)",
		"INSERT INTO t (a) VALUES ('a;''b');", "SELECT FROM", "\xff", "'", "", ";;",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		stmt, err := sql.Parse(input)
		if err == nil && stmt == nil {
			t.Fatal("successful parse returned no statement")
		}
		if err != nil && stmt != nil {
			t.Fatal("failed parse returned a statement")
		}
	})
}
