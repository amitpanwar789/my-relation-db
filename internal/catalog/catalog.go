// This file remembers table definitions for one in-memory database session.
// It validates schemas, preserves column order, and looks up names consistently.
// Registration copies input, and lookups return copies to protect owned metadata.
// The catalog contains no rows and disappears when its session ends.

package catalog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amitpanwar789/my-relation-db/internal/types"
)

type Column struct {
	Name string     `json:"name"`
	Type types.Type `json:"type"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

func (t Table) ColumnIndex(name string) (int, bool) {
	name = strings.ToLower(name)
	for i, column := range t.Columns {
		if column.Name == name {
			return i, true
		}
	}
	return 0, false
}

// Define creates a normalized schema without changing any catalog.
func Define(name string, columns []Column) (Table, error) {
	if name == "" || len(columns) == 0 {
		return Table{}, fmt.Errorf("table name and at least one column are required")
	}
	table := Table{Name: strings.ToLower(name), Columns: slices.Clone(columns)}
	seen := make(map[string]bool)
	for i := range table.Columns {
		column := &table.Columns[i]
		column.Name = strings.ToLower(column.Name)
		if column.Name == "" {
			return Table{}, fmt.Errorf("table %q has an empty column name", table.Name)
		}
		if seen[column.Name] {
			return Table{}, fmt.Errorf("duplicate column %q in table %q", column.Name, table.Name)
		}
		if !column.Type.Valid() {
			return Table{}, fmt.Errorf("unsupported type %q for column %q", column.Type, column.Name)
		}
		seen[column.Name] = true
	}
	return table, nil
}

// Catalog's zero value is an empty, usable catalog. It is not concurrency-safe.
type Catalog struct{ tables map[string]Table }

func (c *Catalog) Lookup(name string) (Table, bool) {
	table, ok := c.tables[strings.ToLower(name)]
	table.Columns = slices.Clone(table.Columns)
	return table, ok
}

func (c *Catalog) Register(table Table) error {
	owned, err := Define(table.Name, table.Columns)
	if err != nil {
		return err
	}
	if _, exists := c.tables[owned.Name]; exists {
		return fmt.Errorf("table %q already exists", owned.Name)
	}
	if c.tables == nil {
		c.tables = make(map[string]Table)
	}
	c.tables[owned.Name] = owned
	return nil
}
