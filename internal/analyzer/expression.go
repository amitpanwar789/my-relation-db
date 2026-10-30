// This file resolves expression names and types before any rows are read.
// It binds columns to positions and gives NULL literals their contextual types.
// Every logical branch is validated even when execution could short-circuit it.
// WHERE accepts only BOOLEAN values, preserving strict typing across statements.

package analyzer

import (
	"fmt"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func analyzeWhere(parsed sql.Expression, table catalog.Table) (*expression.Expr, error) {
	if parsed == nil {
		return nil, nil
	}
	bound, err := bindExpression(parsed, table)
	if err != nil {
		return nil, err
	}
	return requireBoolean(bound)
}

func requireBoolean(expr *expression.Expr) (*expression.Expr, error) {
	expr = typeNull(expr, types.Boolean)
	if expr.Type() != types.Boolean {
		return nil, fmt.Errorf("condition requires BOOLEAN, got %s", expr.Type())
	}
	return expr, nil
}

// Only a bare NULL literal has no type during binding. All other nodes have
// concrete result types; contextual inference never changes a typed column.
func typeNull(expr *expression.Expr, target types.Type) *expression.Expr {
	if expr.Type() == "" {
		return expression.Literal(types.NullValue(target))
	}
	return expr
}

func bindExpression(parsed sql.Expression, table catalog.Table) (*expression.Expr, error) {
	switch parsed := parsed.(type) {
	case *sql.ColumnExpression:
		position, exists := table.ColumnIndex(parsed.Name)
		if !exists {
			return nil, fmt.Errorf("unknown column %q in table %q", parsed.Name, table.Name)
		}
		return expression.Column(position, table.Columns[position].Type), nil
	case *sql.LiteralExpression:
		var target types.Type
		switch parsed.Literal.Kind {
		case sql.LiteralNull:
			return expression.Literal(types.NullValue("")), nil
		case sql.LiteralInteger:
			target = types.Integer
		case sql.LiteralText:
			target = types.Text
		case sql.LiteralBoolean:
			target = types.Boolean
		}
		value, err := convertLiteral(parsed.Literal, target)
		if err != nil {
			return nil, err
		}
		return expression.Literal(value), nil
	case *sql.NullTestExpression:
		operand, err := bindExpression(parsed.Operand, table)
		if err != nil {
			return nil, err
		}
		return expression.IsNull(typeNull(operand, types.Boolean), parsed.Not), nil
	case *sql.BinaryExpression:
		left, err := bindExpression(parsed.Left, table)
		if err != nil {
			return nil, err
		}
		right, err := bindExpression(parsed.Right, table)
		if err != nil {
			return nil, err
		}
		if parsed.Operator == sql.And || parsed.Operator == sql.Or {
			left, err = requireBoolean(left)
			if err != nil {
				return nil, err
			}
			right, err = requireBoolean(right)
			if err != nil {
				return nil, err
			}
		} else {
			// With two NULL literals any comparison is unknown; INTEGER supplies a
			// concrete internal type without implying that NULL is a numeric value.
			if left.Type() == "" && right.Type() == "" {
				left = typeNull(left, types.Integer)
			}
			left = typeNull(left, right.Type())
			right = typeNull(right, left.Type())
			if left.Type() != right.Type() {
				return nil, fmt.Errorf("cannot compare %s and %s", left.Type(), right.Type())
			}
			if left.Type() == types.Boolean && parsed.Operator != sql.Equal && parsed.Operator != sql.NotEqual {
				return nil, fmt.Errorf("BOOLEAN supports only = and <>")
			}
		}
		return expression.Binary(string(parsed.Operator), left, right), nil
	default:
		return nil, fmt.Errorf("unsupported expression %T", parsed)
	}
}
