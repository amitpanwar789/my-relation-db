// This file checks whether SQL tokens follow our supported grammar.
// It reads each statement and builds a matching structured description.
// Unexpected or leftover tokens produce errors showing their location.
// Table existence and value compatibility are checked in a later stage.

package sql

import "fmt"

type parser struct {
	tokens  []Token
	index   int
	nesting int
	depths  map[Expression]int
}

// Parse accepts exactly one statement, optionally terminated with a semicolon.
// It deliberately does not check table existence, types, or INSERT arity.
func Parse(input string) (Statement, error) {
	tokens, err := Tokenize(input)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens}
	var stmt Statement
	switch p.current().Kind {
	case Create:
		stmt, err = p.createTable()
	case Insert:
		stmt, err = p.insert()
	case Select:
		stmt, err = p.selectStatement()
	case Update:
		stmt, err = p.updateStatement()
	case Delete:
		stmt, err = p.deleteStatement()
	default:
		err = p.expected("CREATE, INSERT, SELECT, UPDATE, or DELETE")
	}
	if err != nil {
		return nil, err
	}
	p.match(Semicolon)
	if _, err := p.expect(EOF); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *parser) createTable() (Statement, error) {
	p.match(Create)
	if _, err := p.expect(Table); err != nil {
		return nil, err
	}
	name, err := p.expect(Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(LeftParen); err != nil {
		return nil, err
	}
	stmt := &CreateTableStatement{Table: name.Value}
	for {
		column, err := p.expect(Identifier)
		if err != nil {
			return nil, err
		}
		typeToken := p.current()
		switch typeToken.Kind {
		case IntegerType, TextType, BooleanType:
			p.index++
		default:
			return nil, p.expected("INTEGER, TEXT, or BOOLEAN")
		}
		stmt.Columns = append(stmt.Columns, ColumnDefinition{Name: column.Value, Type: DataType(typeToken.Value)})
		if !p.match(Comma) {
			break
		}
	}
	if _, err := p.expect(RightParen); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *parser) insert() (Statement, error) {
	p.match(Insert)
	if _, err := p.expect(Into); err != nil {
		return nil, err
	}
	name, err := p.expect(Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(LeftParen); err != nil {
		return nil, err
	}
	columns, err := p.nameList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(RightParen); err != nil {
		return nil, err
	}
	if _, err := p.expect(Values); err != nil {
		return nil, err
	}
	if _, err := p.expect(LeftParen); err != nil {
		return nil, err
	}
	stmt := &InsertStatement{Table: name.Value, Columns: columns}
	for {
		value, err := p.literal()
		if err != nil {
			return nil, err
		}
		stmt.Values = append(stmt.Values, value)
		if !p.match(Comma) {
			break
		}
	}
	if _, err := p.expect(RightParen); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *parser) selectStatement() (Statement, error) {
	p.match(Select)
	stmt := &SelectStatement{}
	if p.match(Star) {
		stmt.All = true
	} else {
		if p.current().Kind != Identifier {
			return nil, p.expected("a column name or *")
		}
		columns, err := p.nameList()
		if err != nil {
			return nil, err
		}
		stmt.Columns = columns
	}
	if _, err := p.expect(From); err != nil {
		return nil, err
	}
	name, err := p.expect(Identifier)
	if err != nil {
		return nil, err
	}
	stmt.Table = name.Value
	stmt.Where, err = p.whereClause()
	return stmt, err
}

func (p *parser) nameList() ([]string, error) {
	var names []string
	for {
		name, err := p.expect(Identifier)
		if err != nil {
			return nil, err
		}
		names = append(names, name.Value)
		if !p.match(Comma) {
			return names, nil
		}
	}
}

func (p *parser) literal() (Literal, error) {
	token := p.current()
	value := Literal{Value: token.Value}
	switch token.Kind {
	case Integer:
		value.Kind = LiteralInteger
	case String:
		value.Kind = LiteralText
	case True, False:
		value.Kind = LiteralBoolean
	case Null:
		value.Kind, value.Value = LiteralNull, ""
	default:
		return Literal{}, p.expected("an integer, string, TRUE, FALSE, or NULL")
	}
	p.index++
	return value, nil
}

func (p *parser) current() Token { return p.tokens[p.index] }

func (p *parser) match(kind TokenKind) bool {
	if p.current().Kind != kind {
		return false
	}
	if kind != EOF {
		p.index++
	}
	return true
}

func (p *parser) expect(kind TokenKind) (Token, error) {
	token := p.current()
	if !p.match(kind) {
		return Token{}, p.expected(string(kind))
	}
	return token, nil
}

func (p *parser) expected(description string) error {
	token := p.current()
	found := fmt.Sprintf("%q", token.Lexeme)
	if token.Kind == EOF {
		found = string(EOF)
	}
	return &SyntaxError{token.Pos, fmt.Sprintf("expected %s, found %s", description, found)}
}

func (p *parser) updateStatement() (Statement, error) {
	p.match(Update)
	name, err := p.expect(Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(Set); err != nil {
		return nil, err
	}
	stmt := &UpdateStatement{Table: name.Value}
	for {
		column, err := p.expect(Identifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(Equal); err != nil {
			return nil, err
		}
		value, err := p.literal()
		if err != nil {
			return nil, err
		}
		stmt.Assignments = append(stmt.Assignments, Assignment{Column: column.Value, Value: value})
		if !p.match(Comma) {
			break
		}
	}
	stmt.Where, err = p.whereClause()
	return stmt, err
}

func (p *parser) deleteStatement() (Statement, error) {
	p.match(Delete)
	if _, err := p.expect(From); err != nil {
		return nil, err
	}
	name, err := p.expect(Identifier)
	if err != nil {
		return nil, err
	}
	stmt := &DeleteStatement{Table: name.Value}
	stmt.Where, err = p.whereClause()
	return stmt, err
}
