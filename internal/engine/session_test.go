// This file exercises parsing and semantic checks through a real engine session.
// It checks schema lifetime, isolation, error recovery, and returned snapshots.
// Validated inserts and selects must not pretend to have executed row operations.
// These integration tests use no disk files and retain the syntax error boundary.

package engine_test

import (
	"errors"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/engine"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestSessionLifecycle(t *testing.T) {
	var s engine.Session
	created, err := s.Process("CREATE TABLE users (id INTEGER, name TEXT)")
	if err != nil || created.Status != "created-in-memory" {
		t.Fatalf("CREATE: %+v, %v", created, err)
	}
	created.Command.(*analyzer.CreateTable).Table.Columns[0].Name = "external change"
	for _, query := range []string{
		"INSERT INTO users (name, id) VALUES ('Amit', 42)",
		"SELECT name, id, name FROM users",
		"SELECT * FROM users",
	} {
		result, err := s.Process(query)
		if err != nil || result.Status != "validated-only" {
			t.Fatalf("%s: %+v, %v", query, result, err)
		}
	}
	var independent engine.Session
	if _, err := independent.Process("SELECT * FROM users"); err == nil {
		t.Fatal("new session inherited another session's catalog")
	}
	if _, err := independent.Process("CREATE TABLE users (active BOOLEAN)"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Process("SELECT active FROM users"); err == nil {
		t.Fatal("one session changed another's table definition")
	}
}

func TestFailedCommandsPreserveState(t *testing.T) {
	var s engine.Session
	if _, err := s.Process("CREATE TABLE t (a INTEGER, a TEXT)"); err == nil {
		t.Fatal("invalid CREATE accepted")
	}
	if _, err := s.Process("CREATE TABLE t (a INTEGER)"); err != nil {
		t.Fatalf("invalid CREATE reserved a name: %v", err)
	}
	for _, query := range []string{
		"CREATE TABLE t (other TEXT)",
		"INSERT INTO t (a) VALUES ('wrong')",
		"SELECT missing FROM t",
		"CREATE TABLE syntax_error (a INTEGER); junk",
	} {
		if _, err := s.Process(query); err == nil {
			t.Errorf("invalid command accepted: %s", query)
		}
	}
	if _, err := s.Process("CREATE TABLE syntax_error (a TEXT)"); err != nil {
		t.Fatalf("trailing syntax error mutated catalog: %v", err)
	}
	result, err := s.Process("INSERT INTO t (a) VALUES (42)")
	if err != nil {
		t.Fatal(err)
	}
	if result.Command.(*analyzer.Insert).Row[0] != types.IntegerValue(42) {
		t.Fatal("rejected command changed original schema")
	}
}

func TestErrorKindsAndSnapshotOwnership(t *testing.T) {
	var s engine.Session
	_, err := s.Process("SELECT FROM t")
	var syntax *sql.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("lost syntax error type: %v", err)
	}
	_, err = s.Process("SELECT * FROM t")
	var semantic *analyzer.Error
	if !errors.As(err, &semantic) {
		t.Fatalf("lost semantic error type: %v", err)
	}
	if _, err := s.Process("CREATE TABLE t (a INTEGER, b TEXT)"); err != nil {
		t.Fatal(err)
	}
	first, err := s.Process("SELECT b, a FROM t")
	if err != nil {
		t.Fatal(err)
	}
	bound := first.Command.(*analyzer.Select)
	bound.Table.Columns[0].Name = "changed"
	bound.Columns[0].Name = "changed"
	bound.Positions[0] = 99
	fresh, err := s.Process("SELECT b, a FROM t")
	if err != nil {
		t.Fatal(err)
	}
	got := fresh.Command.(*analyzer.Select)
	if got.Table.Columns[0].Name != "a" || got.Columns[0].Name != "b" || got.Positions[0] != 1 {
		t.Fatal("result mutation leaked into session")
	}
}
