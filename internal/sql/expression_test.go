// This file tests expression syntax, grouping, and the new mutation statements.
// It checks longest-match operators and rejects malformed or unsupported forms.
// Depth boundaries cover parentheses and long left-associated condition trees.
// These syntax tests intentionally need no catalog, stored rows, or type checking.

package sql_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

func TestExpressionOperators(t *testing.T) {
	tokens, err := sql.Tokenize("= <> != < <= > >= WHERE UPDATE SET DELETE AND OR IS NOT")
	if err != nil {
		t.Fatal(err)
	}
	want := []sql.TokenKind{sql.Equal, sql.NotEqual, sql.NotEqual, sql.Less, sql.LessEqual, sql.Greater, sql.GreaterEqual, sql.Where, sql.Update, sql.Set, sql.Delete, sql.And, sql.Or, sql.Is, sql.Not, sql.EOF}
	var got []sql.TokenKind
	for _, token := range tokens {
		got = append(got, token.Kind)
	}
	if !reflect.DeepEqual(got, want) || tokens[2].Lexeme != "!=" {
		t.Fatalf("operators: %+v", tokens)
	}
	for _, op := range []string{"=", "<>", "!=", "<", "<=", ">", ">="} {
		if _, err := sql.Parse("SELECT * FROM t WHERE 1 " + op + " id"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExpressionGrouping(t *testing.T) {
	stmt, err := sql.Parse("SELECT name FROM t WHERE id = 1 OR id = 2 AND active IS NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	root := stmt.(*sql.SelectStatement).Where.(*sql.BinaryExpression)
	if root.Operator != sql.Or || root.Left.(*sql.BinaryExpression).Operator != sql.Equal {
		t.Fatalf("wrong root: %+v", root)
	}
	and := root.Right.(*sql.BinaryExpression)
	if and.Operator != sql.And || !and.Right.(*sql.NullTestExpression).Not {
		t.Fatalf("wrong AND grouping: %+v", and)
	}
	stmt, err = sql.Parse("SELECT * FROM t WHERE (id = 1 OR id = 2) AND active")
	if err != nil {
		t.Fatal(err)
	}
	root = stmt.(*sql.SelectStatement).Where.(*sql.BinaryExpression)
	if root.Operator != sql.And || root.Left.(*sql.BinaryExpression).Operator != sql.Or {
		t.Fatal("parentheses ignored")
	}
}

func TestMutationSyntax(t *testing.T) {
	stmt, err := sql.Parse("UPDATE Users SET name = 'Amit', active = NULL WHERE id = 42;")
	if err != nil {
		t.Fatal(err)
	}
	update := stmt.(*sql.UpdateStatement)
	if update.Kind() != "update" || update.Table != "users" || len(update.Assignments) != 2 || update.Assignments[1].Value.Kind != sql.LiteralNull || update.Where == nil {
		t.Fatalf("update: %+v", update)
	}
	for _, query := range []string{"DELETE FROM t", "DELETE FROM t WHERE a IS NULL", "UPDATE t SET a = 1"} {
		if _, err := sql.Parse(query); err != nil {
			t.Fatal(err)
		}
	}
	stmt, err = sql.Parse("DELETE FROM t WHERE a = 1")
	if err != nil || stmt.Kind() != "delete" {
		t.Fatalf("delete: %+v, %v", stmt, err)
	}
}

func TestExpressionSyntaxErrors(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM t WHERE", "SELECT * FROM t WHERE ()", "SELECT * FROM t WHERE (TRUE",
		"SELECT * FROM t WHERE id =", "SELECT * FROM t WHERE id ! 1", "SELECT * FROM t WHERE id == 1",
		"SELECT * FROM t WHERE id IS TRUE", "SELECT * FROM t WHERE id IS NOT",
		"SELECT * FROM t WHERE 1 < id < 10", "SELECT * FROM t WHERE NOT active",
		"SELECT * FROM t WHERE TRUE AND", "SELECT * FROM t WHERE TRUE OR",
		"UPDATE", "UPDATE t", "UPDATE t SET", "UPDATE t SET a", "UPDATE t SET a =",
		"UPDATE t SET a = 1,", "UPDATE t SET a = b", "UPDATE t SET a = 1 + 1",
		"DELETE", "DELETE FROM", "DELETE t", "DELETE FROM t WHERE", "UPDATE t SET a = 1 WHERE",
	} {
		t.Run(query, func(t *testing.T) {
			stmt, err := sql.Parse(query)
			var syntax *sql.SyntaxError
			if stmt != nil || !errors.As(err, &syntax) {
				t.Fatalf("expected syntax error: %v, %v", stmt, err)
			}
		})
	}
}

func TestExpressionDepthLimit(t *testing.T) {
	for _, n := range []int{sql.MaxExpressionDepth, sql.MaxExpressionDepth + 1} {
		for _, condition := range []string{
			strings.Repeat("(", n) + "TRUE" + strings.Repeat(")", n),
			strings.Repeat("TRUE AND ", n-1) + "TRUE",
			strings.Repeat("FALSE OR ", n-1) + "FALSE",
		} {
			_, err := sql.Parse("SELECT * FROM t WHERE " + condition)
			if (err == nil) != (n == sql.MaxExpressionDepth) {
				t.Fatalf("depth %d: %v", n, err)
			}
		}
	}
}
