// This file tests expression binding and mutation validation against real schemas.
// It checks strict types, contextual NULL, and resolved assignment positions.
// Invalid branches must fail even when runtime evaluation would skip them.
// Analysis uses no stored rows, so validation also applies to empty tables.

package analyzer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestPredicateAndMutationBinding(t *testing.T) {
	c := schema(t)
	for _, condition := range []string{
		"active", "TRUE", "FALSE", "NULL", "id = NULL", "NULL = id", "NULL = NULL",
		"NULL < NULL", "id IS NULL", "active IS NOT NULL", "42 <= id", "id = id",
		"name < 'z'", "active <> FALSE", "active != TRUE", "NULL OR active", "(id = 1 OR id = 2) AND TRUE",
	} {
		command, err := analyze(t, "SELECT name FROM users WHERE "+condition, c)
		if err != nil || command.(*analyzer.Select).Predicate.Type() != types.Boolean {
			t.Fatalf("%s: %+v, %v", condition, command, err)
		}
	}
	command, err := analyze(t, "UPDATE users SET name = 'Amit', id = 9, active = NULL WHERE id = 42", c)
	if err != nil {
		t.Fatal(err)
	}
	update := command.(*analyzer.Update)
	if update.Kind() != "update" || update.Assignments[0].Position != 1 || update.Assignments[1].Position != 0 || update.Assignments[2].Value != types.NullValue(types.Boolean) || update.Predicate == nil {
		t.Fatalf("bound update: %+v", update)
	}
	command, err = analyze(t, "DELETE FROM users WHERE active IS NULL", c)
	if err != nil || command.Kind() != "delete" || command.(*analyzer.Delete).Predicate == nil {
		t.Fatalf("bound delete: %+v, %v", command, err)
	}
	for _, query := range []string{"UPDATE users SET id = 1", "DELETE FROM users"} {
		if _, err := analyze(t, query, c); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPredicateAndMutationSemanticErrors(t *testing.T) {
	c := schema(t)
	for _, tt := range []struct{ query, message string }{
		{"SELECT * FROM users WHERE id", "BOOLEAN"},
		{"SELECT * FROM users WHERE 42", "BOOLEAN"},
		{"SELECT * FROM users WHERE id = '42'", "cannot compare"},
		{"SELECT * FROM users WHERE active < FALSE", "BOOLEAN supports"},
		{"SELECT * FROM users WHERE active < NULL", "BOOLEAN supports"},
		{"SELECT * FROM users WHERE TRUE AND 1", "BOOLEAN"},
		{"SELECT * FROM users WHERE 1 OR TRUE", "BOOLEAN"},
		{"SELECT * FROM users WHERE TRUE OR missing = 1", "unknown column"},
		{"SELECT * FROM users WHERE FALSE AND id = '1'", "cannot compare"},
		{"SELECT * FROM users WHERE id = 9223372036854775808", "out-of-range"},
		{"UPDATE absent SET id = 1", "unknown table"},
		{"DELETE FROM absent", "unknown table"},
		{"DELETE FROM users WHERE missing = 1", "unknown column"},
		{"UPDATE users SET missing = 1", "unknown column"},
		{"UPDATE users SET id = 1, ID = 2", "duplicate SET"},
		{"UPDATE users SET name = 'valid', id = 'wrong'", "expected INTEGER"},
		{"UPDATE users SET id = -9223372036854775809", "out-of-range"},
		{"UPDATE users SET id = 1 WHERE name", "BOOLEAN"},
	} {
		t.Run(tt.query, func(t *testing.T) {
			command, err := analyze(t, tt.query, c)
			var semantic *analyzer.Error
			if command != nil || !errors.As(err, &semantic) || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("%+v, %v; expected %s", command, err, tt.message)
			}
		})
	}
}
