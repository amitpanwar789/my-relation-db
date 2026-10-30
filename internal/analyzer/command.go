// This file describes statements after their names and values are resolved.
// Insert values use table column order; select positions preserve requested order.
// Commands own schema snapshots and do not expose the catalog's mutable state.
// They carry no rows from storage, file locations, or presentation instructions.

package analyzer

import (
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

type Command interface {
	bound()
	Kind() string
}

type CreateTable struct {
	Table catalog.Table `json:"table"`
}

func (*CreateTable) bound()       {}
func (*CreateTable) Kind() string { return "create_table" }

type Insert struct {
	Table catalog.Table `json:"table"`
	Row   []types.Value `json:"prepared_row"`
}

func (*Insert) bound()       {}
func (*Insert) Kind() string { return "insert" }

type Select struct {
	Table     catalog.Table    `json:"table"`
	Positions []int            `json:"positions"`
	Columns   []catalog.Column `json:"columns"`
}

func (*Select) bound()       {}
func (*Select) Kind() string { return "select" }
