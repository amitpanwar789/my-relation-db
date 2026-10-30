// This file describes statements after their names and values are resolved.
// Insert values use table column order; select positions preserve requested order.
// Commands own schema snapshots and do not expose the catalog's mutable state.
// They carry no rows from storage, file locations, or presentation instructions.

package analyzer

import (
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/expression"
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
	Predicate *expression.Expr
	Table     catalog.Table    `json:"table"`
	Positions []int            `json:"positions"`
	Columns   []catalog.Column `json:"columns"`
}

func (*Select) bound()       {}
func (*Select) Kind() string { return "select" }

type Assignment struct {
	Position int
	Value    types.Value
}

type Update struct {
	Table       catalog.Table
	Assignments []Assignment
	Predicate   *expression.Expr
}

func (*Update) bound()       {}
func (*Update) Kind() string { return "update" }

type Delete struct {
	Table     catalog.Table
	Predicate *expression.Expr
}

func (*Delete) bound()       {}
func (*Delete) Kind() string { return "delete" }
