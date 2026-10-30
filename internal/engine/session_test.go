// This file exercises the complete SQL pipeline through engine sessions.
// It verifies typed rows, projections, rejected commands, and independent state.
// Returned metadata and values are mutated to check storage and catalog ownership.
// Syntax and semantic errors retain their original types through orchestration.

package engine_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/engine"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func process(t *testing.T, s *engine.Session, query string) engine.Result {
	t.Helper()
	result, err := s.Process(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return result
}

func TestSessionLifecycle(t *testing.T) {
	var s engine.Session
	created := process(t, &s, "CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN)")
	if created.Status != "created-in-memory" {
		t.Fatalf("CREATE: %+v", created)
	}
	empty := process(t, &s, "SELECT * FROM users")
	if len(empty.Query.Rows) != 0 || len(empty.Query.Columns) != 3 {
		t.Fatalf("empty query: %+v", empty.Query)
	}
	insert := process(t, &s, "INSERT INTO users (name, id) VALUES ('Amit', 42)")
	if insert.Status != "inserted" || (insert.AffectedRows == nil || *insert.AffectedRows != 1) {
		t.Fatalf("INSERT: %+v", insert)
	}
	process(t, &s, "INSERT INTO users (id, name, active) VALUES (43, '', FALSE)")
	selected := process(t, &s, "SELECT name, id, name FROM users")
	if len(selected.Query.Rows) != 2 || selected.Query.Columns[0].Name != "name" || selected.Query.Columns[1].Name != "id" {
		t.Fatalf("projection: %+v", selected.Query)
	}
	assertRows(t, selected.Query.Rows, [][]types.Value{
		{types.TextValue("Amit"), types.IntegerValue(42), types.TextValue("Amit")},
		{types.TextValue(""), types.IntegerValue(43), types.TextValue("")},
	})
	all := process(t, &s, "SELECT * FROM users")
	assertRows(t, all.Query.Rows, [][]types.Value{
		{types.IntegerValue(42), types.TextValue("Amit"), types.NullValue(types.Boolean)},
		{types.IntegerValue(43), types.TextValue(""), types.BooleanValue(false)},
	})
	process(t, &s, "CREATE TABLE other (id INTEGER)")
	process(t, &s, "INSERT INTO other (id) VALUES (99)")
	if len(process(t, &s, "SELECT * FROM users").Query.Rows) != 2 {
		t.Fatal("tables share data")
	}
	var independent engine.Session
	if _, err := independent.Process("SELECT * FROM users"); err == nil {
		t.Fatal("new session inherited a schema")
	}
	process(t, &independent, "CREATE TABLE users (active BOOLEAN)")
	if len(process(t, &independent, "SELECT * FROM users").Query.Rows) != 0 {
		t.Fatal("new session inherited rows")
	}
	if _, err := s.Process("SELECT active FROM other"); err == nil {
		t.Fatal("one table changed another's definition")
	}
	if len(process(t, &s, "SELECT * FROM users").Query.Columns) != 3 {
		t.Fatal("session schemas overlap")
	}
}

// Compare as a multiset: SQL does not promise scan order without ORDER BY.
func assertRows(t *testing.T, got, want [][]types.Value) {
	t.Helper()
	remaining := append([][]types.Value(nil), want...)
	for _, row := range got {
		found := false
		for i, expected := range remaining {
			if reflect.DeepEqual(row, expected) {
				remaining = append(remaining[:i], remaining[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected row: %v; expected %v", row, want)
		}
	}
	if len(remaining) != 0 {
		t.Fatalf("missing rows: %v", remaining)
	}
}

func TestFailedCommandsPreserveRowsAndSchemas(t *testing.T) {
	var s engine.Session
	if _, err := s.Process("CREATE TABLE t (a INTEGER, a TEXT)"); err == nil {
		t.Fatal("duplicate columns accepted")
	}
	process(t, &s, "CREATE TABLE t (a INTEGER)")
	process(t, &s, "INSERT INTO t (a) VALUES (42)")
	for _, query := range []string{
		"CREATE TABLE t (other TEXT)", "INSERT INTO t (a) VALUES ('wrong')",
		"INSERT INTO t (a) VALUES (9223372036854775808)", "INSERT INTO t (a) VALUES (-9223372036854775809)",
		"INSERT INTO t (a, a) VALUES (1, 2)", "INSERT INTO t (a) VALUES (1, 2)",
		"INSERT INTO t (missing) VALUES (1)", "INSERT INTO missing (a) VALUES (1)",
		"SELECT missing FROM t", "INSERT INTO t (a) VALUES (9); junk",
		"CREATE TABLE syntax_error (a INTEGER); junk",
	} {
		if _, err := s.Process(query); err == nil {
			t.Errorf("invalid command accepted: %s", query)
		}
	}
	process(t, &s, "CREATE TABLE syntax_error (a TEXT)")
	assertRows(t, process(t, &s, "SELECT * FROM t").Query.Rows, [][]types.Value{{types.IntegerValue(42)}})
	process(t, &s, "INSERT INTO t (a) VALUES (43)")
	if len(process(t, &s, "SELECT * FROM t").Query.Rows) != 2 {
		t.Fatal("session failed to recover")
	}
}

func TestTypedBoundariesAndResultOwnership(t *testing.T) {
	var s engine.Session
	process(t, &s, "CREATE TABLE t (a INTEGER, b TEXT, c BOOLEAN)")
	for _, query := range []string{
		"INSERT INTO t (a, b, c) VALUES (-9223372036854775808, 'O''Brien; नमस्ते', TRUE)",
		"INSERT INTO t (a, b, c) VALUES (9223372036854775807, '', FALSE)",
		"INSERT INTO t (a, b, c) VALUES (0, NULL, NULL)",
		"INSERT INTO t (a) VALUES (NULL)",
	} {
		process(t, &s, query)
	}
	want := [][]types.Value{
		{types.IntegerValue(math.MinInt64), types.TextValue("O'Brien; नमस्ते"), types.BooleanValue(true)},
		{types.IntegerValue(math.MaxInt64), types.TextValue(""), types.BooleanValue(false)},
		{types.IntegerValue(0), types.NullValue(types.Text), types.NullValue(types.Boolean)},
		{types.NullValue(types.Integer), types.NullValue(types.Text), types.NullValue(types.Boolean)},
	}
	first := process(t, &s, "SELECT * FROM t")
	assertRows(t, first.Query.Rows, want)
	first.Query.Columns[0].Name = "changed"
	first.Query.Rows[0][0] = types.IntegerValue(99)
	first.Query.Rows[1] = nil
	fresh := process(t, &s, "SELECT * FROM t")
	assertRows(t, fresh.Query.Rows, want)
	if fresh.Query.Columns[0].Name != "a" {
		t.Fatal("result changed catalog metadata")
	}
}

func TestErrorKinds(t *testing.T) {
	var s engine.Session
	_, err := s.Process("SELECT FROM t")
	var syntax *sql.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("lost syntax error type: %v", err)
	}
	_, err = s.Process("SELECT * FROM t")
	var semantic *analyzer.Error
	if !errors.As(err, &semantic) {
		t.Fatalf("lost semantic error type: %v", err)
	}
}
