// This file verifies that mutation plans preserve resolved commands and predicates.
// Prepared assignments are copied so changing a command cannot alter its plan.
// Predicate copies protect plans even when a caller replaces a command's node.
// Filtering metadata remains attached to selects before projection is executed.

package planner_test

import (
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestMutationPlans(t *testing.T) {
	predicate := expression.Column(1, types.Boolean)
	command := &analyzer.Update{Table: catalog.Table{Name: "t"}, Predicate: predicate, Assignments: []analyzer.Assignment{{Position: 0, Value: types.IntegerValue(42)}}}
	built, err := planner.Build(command)
	if err != nil {
		t.Fatal(err)
	}
	update := built.(*planner.Update)
	command.Assignments[0].Value = types.IntegerValue(99)
	if update.Table != "t" || update.Predicate == predicate || update.Assignments[0].Value != types.IntegerValue(42) {
		t.Fatal("update plan changed")
	}
	built, err = planner.Build(&analyzer.Delete{Table: catalog.Table{Name: "t"}, Predicate: predicate})
	if err != nil || built.(*planner.Delete).Table != "t" || built.(*planner.Delete).Predicate == predicate {
		t.Fatalf("delete plan: %+v, %v", built, err)
	}
	built, err = planner.Build(&analyzer.Select{Table: catalog.Table{Name: "t"}, Predicate: predicate})
	if err != nil || built.(*planner.Select).Predicate == predicate {
		t.Fatal("select lost filter")
	}
	*predicate = *expression.Literal(types.BooleanValue(false))
	value, err := update.Predicate.Eval([]types.Value{types.IntegerValue(1), types.BooleanValue(true)})
	if err != nil || value != types.BooleanValue(true) {
		t.Fatal("command predicate mutation changed the plan")
	}
}
