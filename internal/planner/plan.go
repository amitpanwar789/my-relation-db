// This file turns validated commands into small descriptions of executable work.
// Plans describe insertion, filtered scans, and prepared update/delete operations.
// Planning copies command slices and predicates without reading or changing rows.
// Names and types are already resolved by analysis, so no schema checks repeat here.

package planner

import (
	"fmt"
	"slices"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

type Plan interface{ plan() }

type Insert struct {
	Table string
	Row   []types.Value
}

func (*Insert) plan() {}

type TableScan struct{ Table string }

// Select projects Positions from every row supplied by Scan.
type Select struct {
	Predicate *expression.Expr
	Scan      TableScan
	Positions []int
	Columns   []catalog.Column
}

func (*Select) plan() {}

// Build accepts row commands only. CREATE remains the session's catalog operation.
func Build(command analyzer.Command) (Plan, error) {
	switch command := command.(type) {
	case *analyzer.Insert:
		return &Insert{Table: command.Table.Name, Row: slices.Clone(command.Row)}, nil
	case *analyzer.Select:
		return &Select{
			Scan:      TableScan{Table: command.Table.Name},
			Positions: slices.Clone(command.Positions),
			Columns:   slices.Clone(command.Columns),
			Predicate: command.Predicate.Clone(),
		}, nil
	case *analyzer.Update:
		return &Update{Table: command.Table.Name, Assignments: slices.Clone(command.Assignments), Predicate: command.Predicate.Clone()}, nil
	case *analyzer.Delete:
		return &Delete{Table: command.Table.Name, Predicate: command.Predicate.Clone()}, nil
	default:
		return nil, fmt.Errorf("cannot plan row command %T", command)
	}
}

type Update struct {
	Table       string
	Assignments []analyzer.Assignment
	Predicate   *expression.Expr
}

func (*Update) plan() {}

type Delete struct {
	Table     string
	Predicate *expression.Expr
}

func (*Delete) plan() {}
