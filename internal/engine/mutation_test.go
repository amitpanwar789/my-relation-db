// This file checks filtering and mutations through complete SQL sessions.
// It compares row multisets so tests do not invent a SQL ordering guarantee.
// Invalid commands and full-table operations verify the agreed state boundaries.
// Duplicate rows and original-row predicates guard against skipped or repeated work.

package engine_test

import (
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/engine"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func populated(t *testing.T) *engine.Session {
	t.Helper()
	s := &engine.Session{}
	process(t, s, "CREATE TABLE t (id INTEGER, name TEXT, active BOOLEAN)")
	for _, query := range []string{
		"INSERT INTO t (id, name, active) VALUES (1, 'Amit', TRUE)",
		"INSERT INTO t (id, name, active) VALUES (2, '', FALSE)",
		"INSERT INTO t (id, name, active) VALUES (2, '', FALSE)",
		"INSERT INTO t (id, name) VALUES (3, 'नमस्ते')",
		"INSERT INTO t (id, name, active) VALUES (0, NULL, NULL)",
	} {
		process(t, s, query)
	}
	return s
}

func TestWhereResults(t *testing.T) {
	for _, tt := range []struct {
		condition string
		ids       []int64
	}{
		{"id >= 2 AND active = FALSE", []int64{2, 2}},
		{"id = 1 OR id = 3 AND active = TRUE", []int64{1}},
		{"(id = 1 OR id = 3) AND active IS NULL", []int64{3}},
		{"active = NULL", nil}, {"NULL = NULL", nil}, {"NULL", nil},
		{"active", []int64{1}}, {"active IS NULL", []int64{3, 0}},
		{"name IS NULL", []int64{0}}, {"name = ''", []int64{2, 2}},
		{"TRUE OR NULL", []int64{1, 2, 2, 3, 0}}, {"FALSE AND NULL", nil},
		{"NULL OR active", []int64{1}}, {"NULL AND active", nil},
		{"active IS NOT NULL", []int64{1, 2, 2}}, {"2 = id", []int64{2, 2}},
		{"id = id", []int64{1, 2, 2, 3, 0}}, {"name < 'Amit'", []int64{2, 2}},
	} {
		t.Run(tt.condition, func(t *testing.T) {
			s := populated(t)
			result := process(t, s, "SELECT id FROM t WHERE "+tt.condition)
			want := make([][]types.Value, len(tt.ids))
			for i, id := range tt.ids {
				want[i] = []types.Value{types.IntegerValue(id)}
			}
			assertRows(t, result.Query.Rows, want)
		})
	}
	s := populated(t)
	result := process(t, s, "SELECT name, name FROM t WHERE id = 1")
	assertRows(t, result.Query.Rows, [][]types.Value{{types.TextValue("Amit"), types.TextValue("Amit")}})
}

func TestMutationCountsAndOriginalRows(t *testing.T) {
	s := populated(t)
	count := func(query string, want int) {
		t.Helper()
		result := process(t, s, query)
		if result.AffectedRows == nil || *result.AffectedRows != want {
			t.Fatalf("%s count: %+v; want %d", query, result, want)
		}
	}
	count("UPDATE t SET id = 9, active = TRUE WHERE id = 2", 2)
	assertRows(t, process(t, s, "SELECT id FROM t").Query.Rows, [][]types.Value{
		{types.IntegerValue(1)}, {types.IntegerValue(9)}, {types.IntegerValue(9)}, {types.IntegerValue(3)}, {types.IntegerValue(0)},
	})
	count("UPDATE t SET id = 9 WHERE id = 9", 2)
	count("UPDATE t SET name = NULL WHERE id = 99", 0)
	count("DELETE FROM t WHERE id = 9", 2)
	count("DELETE FROM t WHERE active = NULL", 0)
	count("DELETE FROM t WHERE active IS NULL", 2)
	count("UPDATE t SET name = 'Changed'", 1)
	assertRows(t, process(t, s, "SELECT name FROM t").Query.Rows, [][]types.Value{{types.TextValue("Changed")}})
	count("DELETE FROM t", 1)
	count("DELETE FROM t", 0)
	count("UPDATE t SET id = 0", 0)
	empty := process(t, s, "SELECT * FROM t WHERE TRUE")
	if len(empty.Query.Columns) != 3 || empty.Query.Rows == nil || len(empty.Query.Rows) != 0 {
		t.Fatalf("empty result: %+v", empty.Query)
	}
	process(t, s, "INSERT INTO t (id) VALUES (7)")
	count("UPDATE t SET id = NULL, active = FALSE WHERE id = 7", 1)
	assertRows(t, process(t, s, "SELECT id, active FROM t").Query.Rows, [][]types.Value{{types.NullValue(types.Integer), types.BooleanValue(false)}})
}

func TestRejectedMutationsAndIsolation(t *testing.T) {
	s := populated(t)
	before := process(t, s, "SELECT * FROM t")
	for _, query := range []string{
		"UPDATE t SET name = 'changed', id = 'wrong'", "UPDATE t SET id = 1, id = 2",
		"DELETE FROM t WHERE TRUE OR missing = 1", "UPDATE t SET id = 4 WHERE 1",
		"DELETE FROM t WHERE id = '2'", "UPDATE t SET id = 9223372036854775808",
		"DELETE FROM t WHERE id = 2; junk", "UPDATE t SET id = id + 1",
	} {
		if _, err := s.Process(query); err == nil {
			t.Errorf("invalid command accepted: %s", query)
		}
		assertRows(t, process(t, s, "SELECT * FROM t").Query.Rows, before.Query.Rows)
	}
	process(t, s, "CREATE TABLE other (id INTEGER)")
	process(t, s, "INSERT INTO other (id) VALUES (1)")
	process(t, s, "DELETE FROM t")
	assertRows(t, process(t, s, "SELECT * FROM other").Query.Rows, [][]types.Value{{types.IntegerValue(1)}})
	independent := populated(t)
	if len(process(t, independent, "SELECT * FROM t").Query.Rows) != 5 {
		t.Fatal("sessions share mutations")
	}
}

func TestPredicateAgreementAcrossOperations(t *testing.T) {
	for _, predicate := range []string{"active", "active IS NULL", "id >= 2 AND active = FALSE", "NULL OR active", "FALSE"} {
		t.Run(predicate, func(t *testing.T) {
			s := populated(t)
			matched := len(process(t, s, "SELECT * FROM t WHERE "+predicate).Query.Rows)
			updated := process(t, s, "UPDATE t SET name = 'matched' WHERE "+predicate)
			deleted := process(t, s, "DELETE FROM t WHERE "+predicate)
			if *updated.AffectedRows != matched || *deleted.AffectedRows != matched || len(process(t, s, "SELECT * FROM t").Query.Rows) != 5-matched {
				t.Fatal("different predicate behavior across operations")
			}
		})
	}
}
