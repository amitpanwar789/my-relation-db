// This file defines the structured descriptions produced by the parser.
// Statements hold table names, values, assignments, and optional conditions.
// The analyzer reads these structures to check names and prepare typed commands.
// These types describe commands without executing them or storing data.

package sql

import "github.com/amitpanwar789/my-relation-db/internal/types"

// Statement is a parsed command, not an executed operation.
type Statement interface {
	statement()
	Kind() string
}

type DataType = types.Type

const (
	TypeInteger = types.Integer
	TypeText    = types.Text
	TypeBoolean = types.Boolean
)

type ColumnDefinition struct {
	Name string   `json:"name"`
	Type DataType `json:"type"`
}

type CreateTableStatement struct {
	Table   string             `json:"table"`
	Columns []ColumnDefinition `json:"columns"`
}

func (*CreateTableStatement) statement()   {}
func (*CreateTableStatement) Kind() string { return "create_table" }

type LiteralKind string

const (
	LiteralInteger LiteralKind = "integer"
	LiteralText    LiteralKind = "text"
	LiteralBoolean LiteralKind = "boolean"
	LiteralNull    LiteralKind = "null"
)

// Value is decoded text or an integer/boolean spelling. Kind disambiguates NULL
// from empty text. Integer range and conversion belong to semantic analysis.
type Literal struct {
	Kind  LiteralKind `json:"kind"`
	Value string      `json:"value"`
}

type InsertStatement struct {
	Table   string    `json:"table"`
	Columns []string  `json:"columns"`
	Values  []Literal `json:"values"`
}

func (*InsertStatement) statement()   {}
func (*InsertStatement) Kind() string { return "insert" }

type SelectStatement struct {
	Where   Expression `json:"where,omitempty"`
	Table   string     `json:"table"`
	All     bool       `json:"all"`
	Columns []string   `json:"columns,omitempty"`
}

func (*SelectStatement) statement()   {}
func (*SelectStatement) Kind() string { return "select" }

type Assignment struct {
	Column string
	Value  Literal
}

type UpdateStatement struct {
	Table       string
	Assignments []Assignment
	Where       Expression
}

func (*UpdateStatement) statement()   {}
func (*UpdateStatement) Kind() string { return "update" }

type DeleteStatement struct {
	Table string
	Where Expression
}

func (*DeleteStatement) statement()   {}
func (*DeleteStatement) Kind() string { return "delete" }
