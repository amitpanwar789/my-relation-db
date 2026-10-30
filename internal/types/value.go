// This file defines the database types and concrete values shared by the engine.
// Constructors keep integers, text, booleans, and typed NULL values distinct.
// Values have no mutable references and can safely cross component boundaries.
// JSON output preserves integers exactly and shows NULL as a separate state.

package types

import (
	"encoding/json"
	"fmt"
)

type Type string

const (
	Integer Type = "INTEGER"
	Text    Type = "TEXT"
	Boolean Type = "BOOLEAN"
)

func (t Type) Valid() bool { return t == Integer || t == Text || t == Boolean }

type Value struct {
	typeName Type
	null     bool
	integer  int64
	text     string
	boolean  bool
}

func IntegerValue(v int64) Value { return Value{typeName: Integer, integer: v} }
func TextValue(v string) Value   { return Value{typeName: Text, text: v} }
func BooleanValue(v bool) Value  { return Value{typeName: Boolean, boolean: v} }
func NullValue(t Type) Value     { return Value{typeName: t, null: true} }
func (v Value) Type() Type       { return v.typeName }
func (v Value) IsNull() bool     { return v.null }

func (v Value) Integer() (int64, bool) { return v.integer, !v.null && v.typeName == Integer }
func (v Value) Text() (string, bool)   { return v.text, !v.null && v.typeName == Text }
func (v Value) Boolean() (bool, bool)  { return v.boolean, !v.null && v.typeName == Boolean }

func (v Value) MarshalJSON() ([]byte, error) {
	if !v.typeName.Valid() {
		return nil, fmt.Errorf("invalid value type %q", v.typeName)
	}
	var data any
	if !v.null {
		switch v.typeName {
		case Integer:
			data = v.integer
		case Text:
			data = v.text
		case Boolean:
			data = v.boolean
		}
	}
	return json.Marshal(struct {
		Type  Type `json:"type"`
		Value any  `json:"value"`
	}{v.typeName, data})
}
