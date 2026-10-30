// This file owns schemas and rows and coordinates commands within one session.
// It parses and analyzes SQL, registers tables, and plans and executes row commands.
// Results contain acknowledgements or query data with independent owned slices.
// Independent sessions have separate state, and no state is written to disk.

package engine

import (
	"github.com/amitpanwar789/my-relation-db/internal/analyzer"
	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/executor"
	"github.com/amitpanwar789/my-relation-db/internal/planner"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
	"github.com/amitpanwar789/my-relation-db/internal/storage"
)

type Session struct {
	catalog catalog.Catalog
	rows    storage.Memory
}

type Result = executor.Result

func (s *Session) Process(input string) (Result, error) {
	stmt, err := sql.Parse(input)
	if err != nil {
		return Result{}, err
	}
	return s.Apply(stmt)
}

// Apply consumes a parsed statement so the shell can reject incomplete input
// before any state changes. Its result owns independent metadata and row slices.
func (s *Session) Apply(stmt sql.Statement) (Result, error) {
	command, err := analyzer.Analyze(stmt, &s.catalog)
	if err != nil {
		return Result{}, err
	}
	if create, ok := command.(*analyzer.CreateTable); ok {
		if err := s.catalog.Register(create.Table); err != nil {
			return Result{}, err
		}
		return Result{Kind: command.Kind(), Status: "created-in-memory"}, nil
	}
	plan, err := planner.Build(command)
	if err != nil {
		return Result{}, err
	}
	return executor.Execute(plan, &s.rows)
}
