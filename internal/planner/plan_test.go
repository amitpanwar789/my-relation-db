// This file checks how validated commands become small execution plans.
// It uses real analyzer output to cover remapping and projection cases.
// Mutating commands and plans verifies that neither owns the other's slices.
// Planning needs no row store and cannot perform insertion or scanning itself.

package planner_test

import (
	"reflect"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestBuildResolvedPlans(t *testing.T) {
	var schemas catalog.Catalog
	err := schemas.Register(catalog.Table{Name: "t", Columns: []catalog.Column{
		{Name: "id", Type: types.Integer}, {Name: "name", Type: types.Text}, {Name: "active", Type: types.Boolean},
	}})
	if err != nil {
		t.Fatal(err)
	}
	bind := func(t *testing.T, query string) analyzer.Command {
		t.Helper()
		stmt, err := sql.Parse(query)
		if err != nil {
			t.Fatal(err)
		}
		command, err := analyzer.Analyze(stmt, &schemas)
		if err != nil {
			t.Fatal(err)
		}
		return command
	}
	command := bind(t, "INSERT INTO t (name, id) VALUES ('Amit', 42)").(*analyzer.Insert)
	plan, err := planner.Build(command)
	if err != nil {
		t.Fatal(err)
	}
	insert := plan.(*planner.Insert)
	want := []types.Value{types.IntegerValue(42), types.TextValue("Amit"), types.NullValue(types.Boolean)}
	if insert.Table != "t" || !reflect.DeepEqual(insert.Row, want) {
		t.Fatalf("unexpected insert: %+v", insert)
	}
	command.Row[0] = types.IntegerValue(99)
	if insert.Row[0] != want[0] {
		t.Fatal("plan shares command row")
	}
	insert.Row[1] = types.TextValue("changed")
	if command.Row[1] != want[1] {
		t.Fatal("command shares plan row")
	}
	for _, tt := range []struct {
		query     string
		positions []int
	}{
		{"SELECT name, id, name FROM t", []int{1, 0, 1}}, {"SELECT * FROM t", []int{0, 1, 2}},
	} {
		t.Run(tt.query, func(t *testing.T) {
			command := bind(t, tt.query).(*analyzer.Select)
			plan, err := planner.Build(command)
			if err != nil {
				t.Fatal(err)
			}
			selected := plan.(*planner.Select)
			if selected.Scan.Table != "t" || !reflect.DeepEqual(selected.Positions, tt.positions) || !reflect.DeepEqual(selected.Columns, command.Columns) {
				t.Fatalf("unexpected select: %+v", selected)
			}
			command.Positions[0] = 99
			command.Columns[0].Name = "changed"
			if selected.Positions[0] != tt.positions[0] || selected.Columns[0].Name == "changed" {
				t.Fatal("plan shares command metadata")
			}
		})
	}
}

func TestUnsupportedCommand(t *testing.T) {
	for _, command := range []analyzer.Command{nil, &analyzer.CreateTable{}} {
		if _, err := planner.Build(command); err == nil {
			t.Fatal("unsupported row command accepted")
		}
	}
}
