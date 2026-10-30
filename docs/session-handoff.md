# Session Handoff: Ready for the Stage 3 Discussion

Updated: 2026-10-30

## Project and workflow

Repository: github.com/amitpanwar789/my-relation-db. Local project directory:
`/home/amit/Timepass Project/relational-db`.

Build a relational database from scratch in Go, primarily for understanding its
internals. Follow the top-down Phase 1 plan, not the original storage-first order.
Read AGENTS.md, docs/progress.md, and the relevant design before work.

For each stage: explain concepts and options -> discuss -> save the agreed design
under docs/ -> implement -> unit/integration tests -> record results. Do not start
Stage 3 code before discussing and agreeing its design. Use clean, idiomatic Go,
SOLID without unnecessary abstractions, and a plain-language 4–5-line explanation
at the top of every source/test file. Save design explanations and examples so the
user can revisit them. Maintain README links and progress status.

## Completed

- Stage 1: tokenizer, recursive-descent parser, AST, semicolon-aware multiline shell,
  positional syntax errors, unit/integration tests. Pushed as ab1b12f.
- Stage 2: shared typed values, in-memory catalog, semantic analyzer, bound commands,
  session coordination, and shell integration. Included in this local checkpoint
  commit; not pushed. The user requested committing, not pushing, this checkpoint.

## Current behavior and contracts

- SQL supports CREATE TABLE, explicit-column single-row INSERT, SELECT * and named
  projections. No WHERE/UPDATE/DELETE yet.
- INTEGER is signed int64 with explicit overflow rejection. TEXT is valid UTF-8;
  BOOLEAN is strict TRUE/FALSE. No implicit conversion between types.
- Every column is nullable; missing INSERT columns become typed NULL. NULL differs
  from zero, false, and empty text. Reordered INSERT values become schema-ordered.
- Duplicate table definitions/CREATE columns/INSERT targets are rejected. Repeated
  SELECT projections are allowed; wildcard projection uses schema order.
- CREATE TABLE registers a schema and reports created-in-memory. INSERT/SELECT
  only resolve/validate and report validated-only. No rows are stored or retrieved.
- Catalog state belongs to one session and disappears on exit. No disk persistence,
  indexes, transactions, concurrency, server, or permissions yet.
- Parser remains schema-independent. Semantic errors identify names/types; current
  ASTs lack source spans, so semantic diagnostics do not invent source locations.

## Code navigation

- internal/types/value.go: shared Type and immutable Value, concrete accessors,
  exact JSON int64 output, typed NULL. SQL DataType aliases the shared definition.
- internal/catalog/catalog.go: ordered Table/Column schemas, Define, Lookup, Register,
  defensive copying, lowercase names, zero-value-usable Catalog.
- internal/analyzer: Analyze(stmt, SchemaLookup) and bound CreateTable/Insert/Select.
  Insert contains Table and prepared Row; Select has Table, Positions, Columns.
  Analysis does not mutate the catalog.
- internal/engine/session.go: zero-value-usable Session. Process parses SQL then
  calls Apply; Apply accepts an AST, analyzes it, and registers CREATE schemas.
  Result contains Kind, Status, Command. No general executor/planner yet.
- internal/shell: one session per Run; parse and check statement completion before
  Apply, so incomplete CREATE cannot change state. Continue after SQL errors;
  propagate stream failures. cmd/db wires stdin/stdout and interactive prompts.

## Verification already passed

- go test ./... -cover: analyzer 94.5%, catalog 100%, engine 92.3%, shell 100%,
  SQL 99.5%, types 100%; command entry point checked by a real-command smoke test.
- go vet ./...; go build ./...; command build to /tmp/relational-db-stage2.
- Real shell: examples/stage-2.sql, NULL/remapping, exact int64, Unicode/quotes,
  projections, eight rejection cases, recovery, fresh session, incomplete CREATE.
- Markdown links/fences, explanatory file headers, gofmt, and git diff --check.
- Go tools require cache access outside the restricted filesystem; use the normal
  approval mechanism if required. No third-party dependencies.

## Next: Stage 3 design discussion

Read docs/stage-3-in-memory-execution-design.md. It is a discussion draft, not an
agreed implementation plan. Reconcile it with the actual Stage 2 contracts above.
Explain how validated INSERT/SELECT become simple plans and execute over in-memory
rows, including ownership/copying, projections, result formatting, table creation
coordination, and tests. Preserve the existing type/NULL/missing-column policies.
Agree the design with the user, save it, then implement when authorized.

Later stages: 4 expressions/mutations, 5 persistent storage, 6 indexes,
7 index-aware planning/EXPLAIN, 8 release verification. See phase-1-plan.md.
