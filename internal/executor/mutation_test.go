// This file injects failures during mutation preparation and publication.
// Stored data must stay unchanged if scanning or evaluating original rows fails.
// A recording store checks that replacement is deferred and zero matches skip it.
// These tests cover failure boundaries independently of the SQL analyzer.

package executor_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/executor"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/storage"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

type recordingStore struct {
	storage.Memory
	scanError, replaceError error
	replacements            int
}

func (s *recordingStore) Scan(table string) ([][]types.Value, error) {
	if s.scanError != nil {
		return nil, s.scanError
	}
	return s.Memory.Scan(table)
}
func (s *recordingStore) Replace(table string, rows [][]types.Value) error {
	s.replacements++
	if s.replaceError != nil {
		return s.replaceError
	}
	return s.Memory.Replace(table, rows)
}

func TestMutationPreparationFailurePreservesRows(t *testing.T) {
	predicate := expression.Binary("=", expression.Column(0, types.Integer), expression.Literal(types.IntegerValue(1)))
	for _, deleting := range []bool{false, true} {
		store := &recordingStore{}
		rows := [][]types.Value{{types.IntegerValue(1)}, {types.TextValue("invalid stored type")}}
		for _, row := range rows {
			if err := store.Append("t", row); err != nil {
				t.Fatal(err)
			}
		}
		var plan planner.Plan = &planner.Update{Table: "t", Predicate: predicate, Assignments: []analyzer.Assignment{{Position: 0, Value: types.IntegerValue(99)}}}
		if deleting {
			plan = &planner.Delete{Table: "t", Predicate: predicate}
		}
		_, err := executor.Execute(plan, store)
		if err == nil || store.replacements != 0 {
			t.Fatal("failed evaluation published partial changes")
		}
		got, err := store.Memory.Scan("t")
		if err != nil || !reflect.DeepEqual(got, rows) {
			t.Fatalf("original rows changed: %v, %v", got, err)
		}
	}
}

func TestMutationStoreFailuresAndZeroCounts(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		for _, failingScan := range []bool{false, true} {
			store := &recordingStore{}
			if err := store.Append("t", []types.Value{types.IntegerValue(1)}); err != nil {
				t.Fatal(err)
			}
			if failingScan {
				store.scanError = storeFailure
			} else {
				store.replaceError = storeFailure
			}
			var plan planner.Plan = &planner.Update{Table: "t", Assignments: []analyzer.Assignment{{Position: 0, Value: types.IntegerValue(2)}}}
			if deleting {
				plan = &planner.Delete{Table: "t"}
			}
			_, err := executor.Execute(plan, store)
			if !errors.Is(err, storeFailure) {
				t.Fatalf("lost failure cause: %v", err)
			}
			wantCalls := 1
			if failingScan {
				wantCalls = 0
			}
			if store.replacements != wantCalls {
				t.Fatalf("unexpected replacement calls: %d", store.replacements)
			}
			rows, _ := store.Memory.Scan("t")
			if rows[0][0] != types.IntegerValue(1) {
				t.Fatal("failure changed rows")
			}
		}
	}
	store := &recordingStore{}
	for _, plan := range []planner.Plan{&planner.Update{Table: "t"}, &planner.Delete{Table: "t"}} {
		result, err := executor.Execute(plan, store)
		if err != nil || result.AffectedRows == nil || *result.AffectedRows != 0 || store.replacements != 0 {
			t.Fatalf("zero-match operation: %+v, %v", result, err)
		}
	}
}

func TestInvalidAssignmentDoesNotPublish(t *testing.T) {
	for _, assignment := range []analyzer.Assignment{
		{Position: -1, Value: types.IntegerValue(1)}, {Position: 1, Value: types.IntegerValue(1)},
		{Position: 0, Value: types.TextValue("wrong")},
	} {
		store := &recordingStore{}
		if err := store.Append("t", []types.Value{types.IntegerValue(42)}); err != nil {
			t.Fatal(err)
		}
		_, err := executor.Execute(&planner.Update{Table: "t", Assignments: []analyzer.Assignment{assignment}}, store)
		if err == nil || store.replacements != 0 {
			t.Fatal("invalid assignment published rows")
		}
	}
	store := &recordingStore{}
	if err := store.Append("t", []types.Value{types.IntegerValue(1)}); err != nil {
		t.Fatal(err)
	}
	_, err := executor.Execute(&planner.Select{Scan: planner.TableScan{Table: "t"}, Predicate: expression.Column(2, types.Boolean)}, store)
	if err == nil {
		t.Fatal("invalid select predicate accepted")
	}
}
