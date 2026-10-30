// This file tests execution independently of parsing and semantic analysis.
// Memory storage checks inserts, empty scans, projections, and result ownership.
// A failing store verifies that insertion and scan errors retain their causes.
// Invalid projection positions return errors instead of panicking on stored rows.

package executor_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/executor"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/storage"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestExecutionAndProjectionOwnership(t *testing.T) {
	var store storage.Memory
	plan := &planner.Select{Scan: planner.TableScan{Table: "t"}, Positions: []int{1, 0, 1}, Columns: []catalog.Column{
		{Name: "name", Type: types.Text}, {Name: "id", Type: types.Integer}, {Name: "name", Type: types.Text},
	}}
	empty, err := executor.Execute(plan, &store)
	if err != nil || empty.Query == nil || empty.Query.Rows == nil || len(empty.Query.Rows) != 0 || len(empty.Query.Columns) != 3 {
		t.Fatalf("empty result: %+v, %v", empty, err)
	}
	insert := &planner.Insert{Table: "t", Row: []types.Value{types.IntegerValue(42), types.TextValue("Amit")}}
	result, err := executor.Execute(insert, &store)
	if err != nil || result.Kind != "insert" || result.Status != "inserted" || (result.AffectedRows == nil || *result.AffectedRows != 1) || result.Query != nil {
		t.Fatalf("insert result: %+v, %v", result, err)
	}
	insert.Row[0] = types.IntegerValue(99)
	selected, err := executor.Execute(plan, &store)
	want := [][]types.Value{{types.TextValue("Amit"), types.IntegerValue(42), types.TextValue("Amit")}}
	if err != nil || selected.Status != "selected" || !reflect.DeepEqual(selected.Query.Rows, want) {
		t.Fatalf("select result: %+v, %v", selected, err)
	}
	selected.Query.Rows[0][0] = types.TextValue("changed")
	selected.Query.Columns[0].Name = "changed"
	fresh, err := executor.Execute(plan, &store)
	if err != nil || !reflect.DeepEqual(fresh.Query.Rows, want) || fresh.Query.Columns[0].Name != "name" {
		t.Fatal("result mutation leaked into plan or store")
	}
}

var storeFailure = errors.New("store failed")

type brokenStore struct{}

func (brokenStore) Replace(string, [][]types.Value) error { return storeFailure }

func (brokenStore) Append(string, []types.Value) error   { return storeFailure }
func (brokenStore) Scan(string) ([][]types.Value, error) { return nil, storeFailure }

func TestStorageErrors(t *testing.T) {
	for _, plan := range []planner.Plan{&planner.Insert{Table: "t"}, &planner.Select{Scan: planner.TableScan{Table: "t"}}} {
		_, err := executor.Execute(plan, brokenStore{})
		if !errors.Is(err, storeFailure) || !strings.Contains(err.Error(), `"t"`) {
			t.Fatalf("lost storage error context: %v", err)
		}
	}
}

func TestInvalidPlans(t *testing.T) {
	var store storage.Memory
	if err := store.Append("t", []types.Value{types.IntegerValue(1)}); err != nil {
		t.Fatal(err)
	}
	for _, position := range []int{-1, 1} {
		_, err := executor.Execute(&planner.Select{Scan: planner.TableScan{Table: "t"}, Positions: []int{position}}, &store)
		if err == nil || !strings.Contains(err.Error(), "outside row") {
			t.Fatalf("invalid projection: %v", err)
		}
	}
	if _, err := executor.Execute(nil, &store); err == nil {
		t.Fatal("nil plan accepted")
	}
}
