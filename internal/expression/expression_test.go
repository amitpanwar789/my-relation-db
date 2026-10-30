// This file verifies three-valued logic independently of SQL parsing and storage.
// It exercises every AND/OR truth-table pair and distinct typed scalar values.
// Integer extremes and text ordering test comparison without subtraction or locale.
// Invalid rows and malformed bound expressions must return useful errors.

package expression_test

import (
	"math"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestThreeValuedLogic(t *testing.T) {
	values := []types.Value{types.BooleanValue(false), types.BooleanValue(true), types.NullValue(types.Boolean)}
	and := [][]int{{0, 0, 0}, {0, 1, 2}, {0, 2, 2}}
	or := [][]int{{0, 1, 2}, {1, 1, 1}, {2, 1, 2}}
	for operator, table := range map[string][][]int{"AND": and, "OR": or} {
		for i, left := range values {
			for j, right := range values {
				expr := expression.Binary(operator, expression.Literal(left), expression.Literal(right))
				got, err := expr.Eval(nil)
				if err != nil || got != values[table[i][j]] {
					t.Fatalf("%v %s %v = %v, %v", left, operator, right, got, err)
				}
				match, err := expression.Matches(expr, nil)
				if err != nil || match != (table[i][j] == 1) {
					t.Fatalf("incorrect match: %v, %v", match, err)
				}
			}
		}
	}
	for _, not := range []bool{false, true} {
		for _, value := range []types.Value{types.NullValue(types.Integer), types.IntegerValue(0), types.TextValue(""), types.BooleanValue(false)} {
			got, err := expression.IsNull(expression.Literal(value), not).Eval(nil)
			if err != nil || got != types.BooleanValue(value.IsNull() != not) {
				t.Fatalf("NULL test: %v, %v", got, err)
			}
		}
	}
	if match, err := expression.Matches(nil, nil); !match || err != nil {
		t.Fatal("missing WHERE must match")
	}
}

func TestComparisons(t *testing.T) {
	for _, tt := range []struct {
		operator    string
		left, right types.Value
		want        bool
	}{
		{"<", types.IntegerValue(math.MinInt64), types.IntegerValue(math.MaxInt64), true},
		{">", types.IntegerValue(math.MaxInt64), types.IntegerValue(math.MinInt64), true},
		{"=", types.IntegerValue(0), types.IntegerValue(0), true},
		{"<>", types.IntegerValue(0), types.IntegerValue(0), false},
		{"<=", types.IntegerValue(1), types.IntegerValue(1), true},
		{">=", types.IntegerValue(2), types.IntegerValue(1), true},
		{"<", types.TextValue("A"), types.TextValue("a"), true},
		{"<", types.TextValue(""), types.TextValue("नमस्ते"), true},
		{"=", types.TextValue("Amit"), types.TextValue("amit"), false},
		{"=", types.BooleanValue(false), types.BooleanValue(false), true},
		{"<>", types.BooleanValue(true), types.BooleanValue(false), true},
	} {
		got, err := expression.Binary(tt.operator, expression.Literal(tt.left), expression.Literal(tt.right)).Eval(nil)
		if err != nil || got != types.BooleanValue(tt.want) {
			t.Fatalf("%+v: %v, %v", tt, got, err)
		}
	}
	for _, op := range []string{"=", "<>", "<", "<=", ">", ">="} {
		for _, pair := range [][2]types.Value{
			{types.IntegerValue(1), types.NullValue(types.Integer)},
			{types.NullValue(types.Integer), types.IntegerValue(1)},
			{types.NullValue(types.Integer), types.NullValue(types.Integer)},
		} {
			got, err := expression.Binary(op, expression.Literal(pair[0]), expression.Literal(pair[1])).Eval(nil)
			if err != nil || got != types.NullValue(types.Boolean) {
				t.Fatalf("NULL comparison: %v, %v", got, err)
			}
		}
	}
}

func TestEvaluationErrorsAndShortCircuit(t *testing.T) {
	column := expression.Column(0, types.Boolean)
	for _, expr := range []*expression.Expr{
		nil, column, expression.Column(-1, types.Integer),
		expression.Binary("AND", expression.Literal(types.IntegerValue(1)), column),
		expression.Binary("OR", expression.Literal(types.BooleanValue(false)), expression.Literal(types.IntegerValue(1))),
		expression.Binary("=", expression.Literal(types.TextValue("1")), expression.Literal(types.IntegerValue(1))),
		expression.Binary("<", expression.Literal(types.BooleanValue(false)), expression.Literal(types.BooleanValue(true))),
		expression.Binary("?", expression.Literal(types.IntegerValue(1)), expression.Literal(types.IntegerValue(1))),
		expression.IsNull(column, false),
	} {
		if _, err := expr.Eval(nil); err == nil {
			t.Fatalf("invalid evaluation accepted: %+v", expr)
		}
	}
	if _, err := column.Eval([]types.Value{types.TextValue("wrong")}); err == nil {
		t.Fatal("wrong stored type accepted")
	}
	if _, err := expression.Matches(expression.Literal(types.IntegerValue(1)), nil); err == nil {
		t.Fatal("integer predicate accepted")
	}
	for _, tt := range []struct {
		op    string
		truth bool
	}{{"AND", false}, {"OR", true}} {
		got, err := expression.Binary(tt.op, expression.Literal(types.BooleanValue(tt.truth)), column).Eval(nil)
		if err != nil || got != types.BooleanValue(tt.truth) {
			t.Fatal("valid short circuit evaluated missing row")
		}
	}
	got, err := column.Eval([]types.Value{types.BooleanValue(true)})
	if err != nil || got != types.BooleanValue(true) || column.Type() != types.Boolean {
		t.Fatal("column evaluation failed")
	}
}

func TestCloneOwnsEveryNode(t *testing.T) {
	child := expression.Literal(types.BooleanValue(true))
	original := expression.Binary("AND", child, expression.Literal(types.BooleanValue(true)))
	owned := original.Clone()
	*child = *expression.Literal(types.BooleanValue(false))
	*original = *expression.Literal(types.BooleanValue(false))
	got, err := owned.Eval(nil)
	if err != nil || got != types.BooleanValue(true) {
		t.Fatal("clone shares expression nodes")
	}
	var absent *expression.Expr
	if absent.Clone() != nil {
		t.Fatal("missing predicate became a condition")
	}
}
