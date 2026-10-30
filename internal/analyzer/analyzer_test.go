// This file tests the meaning checks applied after SQL has been parsed.
// It checks resolution order, strict types, NULL, and integer range boundaries.
// Catalog snapshots verify that analysis itself never applies a CREATE command.
// Syntax-only parser tests remain independent and continue accepting unknown names.

package analyzer_test

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func schema(t *testing.T) *catalog.Catalog {
	t.Helper()
	c := &catalog.Catalog{}
	if err := c.Register(catalog.Table{Name: "users", Columns: []catalog.Column{{Name: "id", Type: types.Integer}, {Name: "name", Type: types.Text}, {Name: "active", Type: types.Boolean}}}); err != nil {
		t.Fatal(err)
	}
	return c
}

func analyze(t *testing.T, input string, c *catalog.Catalog) (analyzer.Command, error) {
	t.Helper()
	stmt, err := sql.Parse(input)
	if err != nil {
		t.Fatalf("test requires valid grammar: %v", err)
	}
	return analyzer.Analyze(stmt, c)
}

func TestInsertValuesAndOrder(t *testing.T) {
	c := schema(t)
	for _, tt := range []struct {
		input string
		want  []types.Value
	}{
		{"INSERT INTO users (name, id) VALUES ('Amit', 42)", []types.Value{types.IntegerValue(42), types.TextValue("Amit"), types.NullValue(types.Boolean)}},
		{"INSERT INTO users (active, name, id) VALUES (FALSE, '', 0)", []types.Value{types.IntegerValue(0), types.TextValue(""), types.BooleanValue(false)}},
		{"INSERT INTO users (id, name, active) VALUES (NULL, NULL, NULL)", []types.Value{types.NullValue(types.Integer), types.NullValue(types.Text), types.NullValue(types.Boolean)}},
		{"INSERT INTO users (id, active) VALUES (-9223372036854775808, TRUE)", []types.Value{types.IntegerValue(math.MinInt64), types.NullValue(types.Text), types.BooleanValue(true)}},
		{"INSERT INTO users (id) VALUES (+9223372036854775807)", []types.Value{types.IntegerValue(math.MaxInt64), types.NullValue(types.Text), types.NullValue(types.Boolean)}},
	} {
		command, err := analyze(t, tt.input, c)
		if err != nil {
			t.Fatal(err)
		}
		got := command.(*analyzer.Insert)
		if !reflect.DeepEqual(got.Row, tt.want) {
			t.Errorf("%s: row = %+v; want %+v", tt.input, got.Row, tt.want)
		}
	}
}

func TestSelectResolution(t *testing.T) {
	c := schema(t)
	for _, tt := range []struct {
		input     string
		positions []int
		names     []string
	}{
		{"SELECT * FROM users", []int{0, 1, 2}, []string{"id", "name", "active"}},
		{"SELECT name, id, name FROM users", []int{1, 0, 1}, []string{"name", "id", "name"}},
	} {
		command, err := analyze(t, tt.input, c)
		if err != nil {
			t.Fatal(err)
		}
		got := command.(*analyzer.Select)
		var names []string
		for _, column := range got.Columns {
			names = append(names, column.Name)
		}
		if !reflect.DeepEqual(got.Positions, tt.positions) || !reflect.DeepEqual(names, tt.names) {
			t.Fatalf("resolution = %+v", got)
		}
		got.Table.Columns[0].Name = "changed"
	}
	table, _ := c.Lookup("users")
	if table.Columns[0].Name != "id" {
		t.Fatal("bound command exposed owned metadata")
	}
}

func TestCreateAnalysisDoesNotMutate(t *testing.T) {
	var c catalog.Catalog
	command, err := analyze(t, "CREATE TABLE t (a INTEGER)", &c)
	if err != nil || command.(*analyzer.CreateTable).Table.Name != "t" {
		t.Fatalf("CREATE analysis: %+v, %v", command, err)
	}
	if _, exists := c.Lookup("t"); exists {
		t.Fatal("analysis mutated catalog")
	}
}

func TestSemanticErrors(t *testing.T) {
	c := schema(t)
	for _, tt := range []struct{ input, message string }{
		{"SELECT id FROM absent", "unknown table"},
		{"SELECT missing FROM users", "unknown column"},
		{"INSERT INTO absent (id) VALUES (1)", "unknown table"},
		{"INSERT INTO users (missing) VALUES (1)", "unknown column"},
		{"INSERT INTO users (id, id) VALUES (1, 2)", "duplicate INSERT column"},
		{"INSERT INTO users (id, name) VALUES (1)", "supplies 1 values"},
		{"INSERT INTO users (id) VALUES (1, 2)", "supplies 2 values"},
		{"INSERT INTO users (id) VALUES ('42')", "expected INTEGER, got text"},
		{"INSERT INTO users (name) VALUES (42)", "expected TEXT, got integer"},
		{"INSERT INTO users (active) VALUES (1)", "expected BOOLEAN, got integer"},
		{"INSERT INTO users (active) VALUES ('TRUE')", "expected BOOLEAN, got text"},
		{"INSERT INTO users (id) VALUES (FALSE)", "expected INTEGER, got boolean"},
		{"INSERT INTO users (id) VALUES (9223372036854775808)", "out-of-range INTEGER"},
		{"INSERT INTO users (id) VALUES (-9223372036854775809)", "out-of-range INTEGER"},
		{"CREATE TABLE users (other TEXT)", "already exists"},
		{"CREATE TABLE t (a INTEGER, A TEXT)", "duplicate column"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			command, err := analyze(t, tt.input, c)
			var semantic *analyzer.Error
			if command != nil || !errors.As(err, &semantic) || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("got %+v, %v; want semantic error containing %q", command, err, tt.message)
			}
		})
	}
	table, _ := c.Lookup("users")
	if len(table.Columns) != 3 || table.Columns[0].Name != "id" {
		t.Fatal("rejected commands changed catalog")
	}
}
