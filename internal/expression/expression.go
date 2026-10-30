// This file evaluates resolved expressions against one original row.
// Nodes hide their fields, and cloned trees isolate command and plan ownership.
// Boolean NULL represents unknown, and only true conditions match a row.
// Comparison uses direct ordering, avoiding arithmetic overflow at integer limits.

package expression

import (
	"cmp"
	"fmt"

	"github.com/amitpanwar789/my-relation-db/internal/types"
)

type kind uint8

const (
	literal kind = iota
	column
	binary
	nullTest
)

// Expr hides its node fields. Construct it after semantic validation, and clone
// it when transferring ownership because a caller can replace a whole Expr value.
type Expr struct {
	kind        kind
	typeName    types.Type
	value       types.Value
	position    int
	operator    string
	left, right *Expr
	not         bool
}

func Literal(value types.Value) *Expr {
	return &Expr{kind: literal, typeName: value.Type(), value: value}
}
func Column(position int, typeName types.Type) *Expr {
	return &Expr{kind: column, position: position, typeName: typeName}
}
func Binary(operator string, left, right *Expr) *Expr {
	return &Expr{kind: binary, typeName: types.Boolean, operator: operator, left: left, right: right}
}
func IsNull(operand *Expr, not bool) *Expr {
	return &Expr{kind: nullTest, typeName: types.Boolean, left: operand, not: not}
}
func (e *Expr) Type() types.Type { return e.typeName }

func (e *Expr) Clone() *Expr {
	if e == nil {
		return nil
	}
	owned := *e
	owned.left = e.left.Clone()
	owned.right = e.right.Clone()
	return &owned
}

func Matches(predicate *Expr, row []types.Value) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	value, err := predicate.Eval(row)
	if err != nil {
		return false, err
	}
	if value.Type() != types.Boolean {
		return false, fmt.Errorf("condition must be BOOLEAN")
	}
	truth, _ := value.Boolean()
	return truth && !value.IsNull(), nil
}

func (e *Expr) Eval(row []types.Value) (types.Value, error) {
	if e == nil {
		return types.Value{}, fmt.Errorf("missing expression")
	}
	switch e.kind {
	case literal:
		return e.value, nil
	case column:
		if e.position < 0 || e.position >= len(row) {
			return types.Value{}, fmt.Errorf("column position %d outside row of length %d", e.position, len(row))
		}
		value := row[e.position]
		if value.Type() != e.typeName {
			return types.Value{}, fmt.Errorf("column position %d: expected %s, got %s", e.position, e.typeName, value.Type())
		}
		return value, nil
	case nullTest:
		value, err := e.left.Eval(row)
		if err != nil {
			return types.Value{}, err
		}
		return types.BooleanValue(value.IsNull() != e.not), nil
	case binary:
		left, err := e.left.Eval(row)
		if err != nil {
			return types.Value{}, err
		}
		if e.operator == "AND" || e.operator == "OR" {
			return e.logical(left, row)
		}
		right, err := e.right.Eval(row)
		if err != nil {
			return types.Value{}, err
		}
		return compare(e.operator, left, right)
	default:
		return types.Value{}, fmt.Errorf("invalid expression kind")
	}
}

func (e *Expr) logical(left types.Value, row []types.Value) (types.Value, error) {
	if left.Type() != types.Boolean {
		return types.Value{}, fmt.Errorf("%s requires BOOLEAN operands", e.operator)
	}
	l, _ := left.Boolean()
	if !left.IsNull() && (e.operator == "AND" && !l || e.operator == "OR" && l) {
		return left, nil
	}
	right, err := e.right.Eval(row)
	if err != nil {
		return types.Value{}, err
	}
	if right.Type() != types.Boolean {
		return types.Value{}, fmt.Errorf("%s requires BOOLEAN operands", e.operator)
	}
	r, _ := right.Boolean()
	if !right.IsNull() && (e.operator == "AND" && !r || e.operator == "OR" && r) {
		return right, nil
	}
	if left.IsNull() || right.IsNull() {
		return types.NullValue(types.Boolean), nil
	}
	if e.operator == "AND" {
		return types.BooleanValue(l && r), nil
	}
	return types.BooleanValue(l || r), nil
}

func compare(operator string, left, right types.Value) (types.Value, error) {
	if left.Type() != right.Type() || !left.Type().Valid() {
		return types.Value{}, fmt.Errorf("comparison requires matching supported types")
	}
	if left.IsNull() || right.IsNull() {
		return types.NullValue(types.Boolean), nil
	}
	var order int
	switch left.Type() {
	case types.Integer:
		l, _ := left.Integer()
		r, _ := right.Integer()
		order = cmp.Compare(l, r)
	case types.Text:
		l, _ := left.Text()
		r, _ := right.Text()
		order = cmp.Compare(l, r)
	case types.Boolean:
		if operator != "=" && operator != "<>" {
			return types.Value{}, fmt.Errorf("BOOLEAN supports only = and <>")
		}
		l, _ := left.Boolean()
		r, _ := right.Boolean()
		if l != r {
			order = 1
		}
	}
	var result bool
	switch operator {
	case "=":
		result = order == 0
	case "<>":
		result = order != 0
	case "<":
		result = order < 0
	case "<=":
		result = order <= 0
	case ">":
		result = order > 0
	case ">=":
		result = order >= 0
	default:
		return types.Value{}, fmt.Errorf("unsupported comparison %q", operator)
	}
	return types.BooleanValue(result), nil
}
