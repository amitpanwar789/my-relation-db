// This file executes insert and scan/projection plans against a row store.
// It returns acknowledgements or owned query results without printing anything.
// The small store interface keeps query behavior independent of memory or files.
// Storage errors gain operation context, and invalid projections return errors.

package executor

import (
	"fmt"
	"slices"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

// RowStore accepts resolved table names and prepared rows. Append must retain an
// owned copy; Scan returns an owned snapshot. Missing collections are empty.
// Errors must be returned to the caller; no rollback guarantee is implied.
type RowStore interface {
	Append(table string, row []types.Value) error
	Scan(table string) ([][]types.Value, error)
}

type QueryResult struct {
	Columns []catalog.Column `json:"columns"`
	Rows    [][]types.Value  `json:"rows"`
}

type Result struct {
	Kind         string       `json:"kind"`
	Status       string       `json:"status"`
	AffectedRows int          `json:"affected_rows,omitempty"`
	Query        *QueryResult `json:"query,omitempty"`
}

func Execute(plan planner.Plan, store RowStore) (Result, error) {
	switch plan := plan.(type) {
	case *planner.Insert:
		if err := store.Append(plan.Table, plan.Row); err != nil {
			return Result{}, fmt.Errorf("insert into %q: %w", plan.Table, err)
		}
		return Result{Kind: "insert", Status: "inserted", AffectedRows: 1}, nil
	case *planner.Select:
		rows, err := store.Scan(plan.Scan.Table)
		if err != nil {
			return Result{}, fmt.Errorf("scan %q: %w", plan.Scan.Table, err)
		}
		projected := make([][]types.Value, 0, len(rows))
		for _, row := range rows {
			output := make([]types.Value, len(plan.Positions))
			for i, position := range plan.Positions {
				if position < 0 || position >= len(row) {
					return Result{}, fmt.Errorf("scan %q: column position %d outside row of length %d", plan.Scan.Table, position, len(row))
				}
				output[i] = row[position]
			}
			projected = append(projected, output)
		}
		return Result{Kind: "select", Status: "selected", Query: &QueryResult{
			Columns: slices.Clone(plan.Columns), Rows: projected,
		}}, nil
	default:
		return Result{}, fmt.Errorf("cannot execute plan %T", plan)
	}
}
