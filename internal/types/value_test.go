// This file checks that database values keep their type and NULL state.
// It verifies concrete accessors and exact JSON for integer boundary values.
// Empty text, zero, and false are tested separately from typed NULL values.
// These tests do not depend on SQL parsing, catalogs, or session state.

package types_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestValues(t *testing.T) {
	for _, tt := range []struct {
		value types.Value
		json  string
	}{
		{types.IntegerValue(math.MaxInt64), `{"type":"INTEGER","value":9223372036854775807}`},
		{types.IntegerValue(math.MinInt64), `{"type":"INTEGER","value":-9223372036854775808}`},
		{types.IntegerValue(0), `{"type":"INTEGER","value":0}`},
		{types.TextValue(""), `{"type":"TEXT","value":""}`},
		{types.TextValue("नमस्ते"), `{"type":"TEXT","value":"नमस्ते"}`},
		{types.BooleanValue(false), `{"type":"BOOLEAN","value":false}`},
		{types.BooleanValue(true), `{"type":"BOOLEAN","value":true}`},
		{types.NullValue(types.Integer), `{"type":"INTEGER","value":null}`},
		{types.NullValue(types.Text), `{"type":"TEXT","value":null}`},
		{types.NullValue(types.Boolean), `{"type":"BOOLEAN","value":null}`},
	} {
		got, err := json.Marshal(tt.value)
		if err != nil || string(got) != tt.json {
			t.Errorf("JSON = %s, %v; want %s", got, err, tt.json)
		}
		_, integer := tt.value.Integer()
		_, text := tt.value.Text()
		_, boolean := tt.value.Boolean()
		if integer != (tt.value.Type() == types.Integer && !tt.value.IsNull()) || text != (tt.value.Type() == types.Text && !tt.value.IsNull()) || boolean != (tt.value.Type() == types.Boolean && !tt.value.IsNull()) {
			t.Errorf("accessors disagree with type/NULL state: %+v", tt.value)
		}
	}
	if n, ok := types.IntegerValue(42).Integer(); !ok || n != 42 {
		t.Fatal("integer accessor lost value")
	}
	if s, ok := types.TextValue("Amit").Text(); !ok || s != "Amit" {
		t.Fatal("text accessor lost value")
	}
	if b, ok := types.BooleanValue(true).Boolean(); !ok || !b {
		t.Fatal("boolean accessor lost value")
	}
	if _, err := json.Marshal(types.Value{}); err == nil {
		t.Fatal("zero value should not masquerade as a valid database value")
	}
}
