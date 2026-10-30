// This file owns the catalog and coordinates commands within one session.
// It parses SQL, requests semantic analysis, and registers validated tables.
// Inserts and selects return validated commands without storing or reading rows.
// Independent sessions have separate state, and no state is written to disk.

package engine

import (
	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

type Session struct{ catalog catalog.Catalog }

type Result struct {
	Kind    string           `json:"kind"`
	Status  string           `json:"status"`
	Command analyzer.Command `json:"command"`
}

func (s *Session) Process(input string) (Result, error) {
	stmt, err := sql.Parse(input)
	if err != nil {
		return Result{}, err
	}
	return s.Apply(stmt)
}

// Apply consumes a parsed statement so the shell can reject incomplete input
// before any catalog mutation. Its result owns independent metadata snapshots.
func (s *Session) Apply(stmt sql.Statement) (Result, error) {
	command, err := analyzer.Analyze(stmt, &s.catalog)
	if err != nil {
		return Result{}, err
	}
	status := "validated-only"
	if create, ok := command.(*analyzer.CreateTable); ok {
		if err := s.catalog.Register(create.Table); err != nil {
			return Result{}, err
		}
		status = "created-in-memory"
	}
	return Result{Kind: command.Kind(), Status: status, Command: command}, nil
}
