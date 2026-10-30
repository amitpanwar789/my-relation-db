// This file holds rows in memory for one database session.
// Rows are grouped by table name and kept as typed values in schema order.
// Appends, scans, and replacements copy rows to protect owned table data.
// Schemas live in the catalog; a missing row collection represents an empty table.

package storage

import (
	"slices"

	"github.com/amitpanwar789/my-relation-db/internal/types"
)

// Memory's zero value is usable. Callers resolve table names against the catalog
// before accessing it. It has no persistence or concurrent-access guarantees.
type Memory struct{ tables map[string][][]types.Value }

func (m *Memory) Append(table string, row []types.Value) error {
	if m.tables == nil {
		m.tables = make(map[string][][]types.Value)
	}
	m.tables[table] = append(m.tables[table], slices.Clone(row))
	return nil
}

// Scan returns an owned snapshot, including an empty slice for an empty table.
func (m *Memory) Scan(table string) ([][]types.Value, error) {
	stored := m.tables[table]
	rows := make([][]types.Value, len(stored))
	for i, row := range stored {
		rows[i] = slices.Clone(row)
	}
	return rows, nil
}

// Replace prepares an owned collection before publishing it in one map assignment.
// This memory-only behavior is not a persistence or crash-recovery guarantee.
func (m *Memory) Replace(table string, rows [][]types.Value) error {
	owned := make([][]types.Value, len(rows))
	for i, row := range rows {
		owned[i] = slices.Clone(row)
	}
	if m.tables == nil {
		m.tables = make(map[string][][]types.Value)
	}
	m.tables[table] = owned
	return nil
}
