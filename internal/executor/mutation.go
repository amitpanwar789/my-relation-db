// This file prepares UPDATE and DELETE results using original table snapshots.
// Conditions are evaluated before assignments, and deletion builds survivor rows.
// No replacement is published until every row has been processed successfully.
// Counts include all matches, even updates that assign an already existing value.

package executor

import (
	"fmt"
	"slices"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func mutate(table string, predicate *expression.Expr, assignments []analyzer.Assignment, deleting bool, store RowStore) (Result, error) {
	kind, status := "update", "updated"
	if deleting {
		kind, status = "delete", "deleted"
	}
	rows, err := store.Scan(table)
	if err != nil {
		return Result{}, fmt.Errorf("%s scan %q: %w", kind, table, err)
	}
	replacement := make([][]types.Value, 0, len(rows))
	count := 0
	for _, row := range rows {
		match, err := expression.Matches(predicate, row)
		if err != nil {
			return Result{}, fmt.Errorf("%s filter %q: %w", kind, table, err)
		}
		if !match {
			replacement = append(replacement, row)
			continue
		}
		count++
		if deleting {
			continue
		}
		updated := slices.Clone(row)
		for _, assignment := range assignments {
			if assignment.Position < 0 || assignment.Position >= len(updated) {
				return Result{}, fmt.Errorf("update %q: assignment position %d outside row of length %d", table, assignment.Position, len(updated))
			}
			if updated[assignment.Position].Type() != assignment.Value.Type() {
				return Result{}, fmt.Errorf("update %q: assignment type does not match stored column", table)
			}
			updated[assignment.Position] = assignment.Value
		}
		replacement = append(replacement, updated)
	}
	// Avoid unnecessary publication when no rows matched, but still return a
	// successful zero count. Other store implementations must document failures.
	if count > 0 {
		if err := store.Replace(table, replacement); err != nil {
			return Result{}, fmt.Errorf("%s replace %q: %w", kind, table, err)
		}
	}
	return mutationResult(kind, status, count), nil
}
