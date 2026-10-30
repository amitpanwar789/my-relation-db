// This file checks parsed SQL against the tables known to a session.
// It resolves column names, checks value types, and prepares concrete values.
// Analysis only reads schemas; applying a CREATE command belongs to the engine.
// Errors describe the affected names or types without inventing source positions.

package analyzer

import (
	"fmt"
	"strconv"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

// SchemaLookup is the only catalog operation required by analysis.
type SchemaLookup interface {
	Lookup(string) (catalog.Table, bool)
}

type Error struct{ Message string }

func (e *Error) Error() string { return "semantic error: " + e.Message }

func Analyze(stmt sql.Statement, schemas SchemaLookup) (Command, error) {
	var command Command
	var err error
	switch s := stmt.(type) {
	case *sql.CreateTableStatement:
		command, err = analyzeCreate(s, schemas)
	case *sql.InsertStatement:
		command, err = analyzeInsert(s, schemas)
	case *sql.SelectStatement:
		command, err = analyzeSelect(s, schemas)
	default:
		err = fmt.Errorf("unsupported statement %T", stmt)
	}
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	return command, nil
}

func analyzeCreate(stmt *sql.CreateTableStatement, schemas SchemaLookup) (Command, error) {
	columns := make([]catalog.Column, len(stmt.Columns))
	for i, column := range stmt.Columns {
		columns[i] = catalog.Column{Name: column.Name, Type: column.Type}
	}
	table, err := catalog.Define(stmt.Table, columns)
	if err != nil {
		return nil, err
	}
	if _, exists := schemas.Lookup(table.Name); exists {
		return nil, fmt.Errorf("table %q already exists", table.Name)
	}
	return &CreateTable{Table: table}, nil
}

func findTable(schemas SchemaLookup, name string) (catalog.Table, error) {
	table, exists := schemas.Lookup(name)
	if !exists {
		return catalog.Table{}, fmt.Errorf("unknown table %q", name)
	}
	return table, nil
}

func analyzeInsert(stmt *sql.InsertStatement, schemas SchemaLookup) (Command, error) {
	table, err := findTable(schemas, stmt.Table)
	if err != nil {
		return nil, err
	}
	if len(stmt.Columns) != len(stmt.Values) {
		return nil, fmt.Errorf("INSERT into %q names %d columns but supplies %d values", table.Name, len(stmt.Columns), len(stmt.Values))
	}
	row := make([]types.Value, len(table.Columns))
	for i, column := range table.Columns {
		row[i] = types.NullValue(column.Type)
	}
	seen := make(map[int]bool)
	for i, name := range stmt.Columns {
		position, exists := table.ColumnIndex(name)
		if !exists {
			return nil, fmt.Errorf("unknown column %q in table %q", name, table.Name)
		}
		if seen[position] {
			return nil, fmt.Errorf("duplicate INSERT column %q in table %q", name, table.Name)
		}
		seen[position] = true
		value, err := convertLiteral(stmt.Values[i], table.Columns[position].Type)
		if err != nil {
			return nil, fmt.Errorf("table %q, column %q: %w", table.Name, name, err)
		}
		row[position] = value
	}
	return &Insert{Table: table, Row: row}, nil
}

func analyzeSelect(stmt *sql.SelectStatement, schemas SchemaLookup) (Command, error) {
	table, err := findTable(schemas, stmt.Table)
	if err != nil {
		return nil, err
	}
	command := &Select{Table: table}
	if stmt.All {
		for i, column := range table.Columns {
			command.Positions = append(command.Positions, i)
			command.Columns = append(command.Columns, column)
		}
	} else {
		for _, name := range stmt.Columns {
			position, exists := table.ColumnIndex(name)
			if !exists {
				return nil, fmt.Errorf("unknown column %q in table %q", name, table.Name)
			}
			command.Positions = append(command.Positions, position)
			command.Columns = append(command.Columns, table.Columns[position])
		}
	}
	return command, nil
}

func convertLiteral(literal sql.Literal, target types.Type) (types.Value, error) {
	if literal.Kind == sql.LiteralNull {
		return types.NullValue(target), nil
	}
	switch {
	case target == types.Integer && literal.Kind == sql.LiteralInteger:
		value, err := strconv.ParseInt(literal.Value, 10, 64)
		if err != nil {
			return types.Value{}, fmt.Errorf("invalid or out-of-range INTEGER %q (signed 64-bit required)", literal.Value)
		}
		return types.IntegerValue(value), nil
	case target == types.Text && literal.Kind == sql.LiteralText:
		return types.TextValue(literal.Value), nil
	case target == types.Boolean && literal.Kind == sql.LiteralBoolean:
		return types.BooleanValue(literal.Value == "TRUE"), nil
	default:
		return types.Value{}, fmt.Errorf("expected %s, got %s literal", target, literal.Kind)
	}
}
