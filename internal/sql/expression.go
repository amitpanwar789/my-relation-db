// This file describes and parses conditions without knowing any table schemas.
// OR, AND, comparisons, and primary expressions establish the grouping rules.
// Parentheses and expression tree depth are bounded to protect later recursion.
// Analysis resolves names and types after these syntax structures are built.

package sql

import "fmt"

type Expression interface{ expression() }
type ColumnExpression struct{ Name string }
type LiteralExpression struct{ Literal Literal }
type BinaryExpression struct {
	Operator    TokenKind
	Left, Right Expression
}
type NullTestExpression struct {
	Operand Expression
	Not     bool
}

func (*ColumnExpression) expression()   {}
func (*LiteralExpression) expression()  {}
func (*BinaryExpression) expression()   {}
func (*NullTestExpression) expression() {}

const MaxExpressionDepth = 256

func (p *parser) whereClause() (Expression, error) {
	if !p.match(Where) {
		return nil, nil
	}
	return p.disjunction()
}

func (p *parser) disjunction() (Expression, error) {
	return p.logical(Or, p.conjunction)
}

func (p *parser) conjunction() (Expression, error) {
	return p.logical(And, p.comparison)
}

func (p *parser) logical(operator TokenKind, next func() (Expression, error)) (Expression, error) {
	left, err := next()
	if err != nil {
		return nil, err
	}
	for p.match(operator) {
		right, err := next()
		if err != nil {
			return nil, err
		}
		left, err = p.node(&BinaryExpression{Operator: operator, Left: left, Right: right}, left, right)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *parser) comparison() (Expression, error) {
	left, err := p.primary()
	if err != nil {
		return nil, err
	}
	switch operator := p.current().Kind; operator {
	case Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual:
		p.index++
		right, err := p.primary()
		if err != nil {
			return nil, err
		}
		return p.node(&BinaryExpression{Operator: operator, Left: left, Right: right}, left, right)
	case Is:
		p.index++
		not := p.match(Not)
		if _, err := p.expect(Null); err != nil {
			return nil, err
		}
		return p.node(&NullTestExpression{Operand: left, Not: not}, left)
	default:
		return left, nil
	}
}

func (p *parser) primary() (Expression, error) {
	if p.match(LeftParen) {
		p.nesting++
		defer func() { p.nesting-- }()
		if p.nesting > MaxExpressionDepth {
			return nil, p.depthError()
		}
		expr, err := p.disjunction()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(RightParen); err != nil {
			return nil, err
		}
		return expr, nil
	}
	if p.current().Kind == Identifier {
		name := p.current().Value
		p.index++
		return &ColumnExpression{Name: name}, nil
	}
	literal, err := p.literal()
	if err != nil {
		return nil, err
	}
	return &LiteralExpression{Literal: literal}, nil
}

// Bound both parentheses and left-associated trees: long AND chains also recurse
// during analysis/evaluation even when the SQL contains no parentheses.
func (p *parser) node(expr Expression, children ...Expression) (Expression, error) {
	depth := 1
	for _, child := range children {
		childDepth := p.depths[child]
		if childDepth == 0 {
			childDepth = 1
		}
		depth = max(depth, childDepth+1)
	}
	if depth > MaxExpressionDepth {
		return nil, p.depthError()
	}
	if p.depths == nil {
		p.depths = make(map[Expression]int)
	}
	p.depths[expr] = depth
	return expr, nil
}

func (p *parser) depthError() error {
	return &SyntaxError{Pos: p.current().Pos, Message: fmt.Sprintf("expression depth exceeds %d", MaxExpressionDepth)}
}
