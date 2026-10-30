// This file tests registration and lookup of in-memory table definitions.
// It checks normalization, ordered columns, duplicate rejection, and invalid types.
// Mutation checks verify that callers cannot change catalog-owned metadata.
// Failed registrations must not reserve names or disturb existing definitions.

package catalog_test

import (
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestCatalogOwnershipAndLookup(t *testing.T) {
	var c catalog.Catalog
	columns := []catalog.Column{{Name: "ID", Type: types.Integer}, {Name: "Name", Type: types.Text}}
	if err := c.Register(catalog.Table{Name: "Users", Columns: columns}); err != nil {
		t.Fatal(err)
	}
	columns[0].Name = "changed"
	table, ok := c.Lookup("USERS")
	if !ok || table.Name != "users" || table.Columns[0].Name != "id" || table.Columns[1].Name != "name" {
		t.Fatalf("bad snapshot: %+v", table)
	}
	if i, ok := table.ColumnIndex("NAME"); !ok || i != 1 {
		t.Fatal("column lookup lost normalization/order")
	}
	if _, ok := table.ColumnIndex("missing"); ok {
		t.Fatal("missing column found")
	}
	table.Columns[0].Name = "changed again"
	fresh, _ := c.Lookup("users")
	if fresh.Columns[0].Name != "id" {
		t.Fatal("lookup exposed catalog-owned slice")
	}
	if err := c.Register(catalog.Table{Name: "USERS", Columns: []catalog.Column{{Name: "other", Type: types.Text}}}); err == nil {
		t.Fatal("duplicate registration succeeded")
	}
	fresh, _ = c.Lookup("users")
	if len(fresh.Columns) != 2 || fresh.Columns[0].Name != "id" {
		t.Fatal("duplicate registration changed the original")
	}
	if _, ok := c.Lookup("absent"); ok {
		t.Fatal("absent table found")
	}
}

func TestInvalidSchemasDoNotRegister(t *testing.T) {
	for _, table := range []catalog.Table{
		{Name: "t"},
		{Name: "", Columns: []catalog.Column{{Name: "a", Type: types.Text}}},
		{Name: "t", Columns: []catalog.Column{{Name: "a", Type: types.Integer}, {Name: "A", Type: types.Text}}},
		{Name: "t", Columns: []catalog.Column{{Name: "a", Type: "UNKNOWN"}}},
		{Name: "t", Columns: []catalog.Column{{Name: "", Type: types.Text}}},
	} {
		var c catalog.Catalog
		if err := c.Register(table); err == nil {
			t.Errorf("registered invalid schema: %+v", table)
		}
		if _, exists := c.Lookup(table.Name); exists {
			t.Fatal("invalid schema left catalog state")
		}
		if err := c.Register(catalog.Table{Name: "t", Columns: []catalog.Column{{Name: "a", Type: types.Text}}}); err != nil {
			t.Fatalf("failed registration reserved name: %v", err)
		}
	}
}
