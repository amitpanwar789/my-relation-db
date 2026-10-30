# Session Handoff: Stage 4 Implemented

Updated: 2026-10-30

## Project and workflow

Repository: github.com/amitpanwar789/my-relation-db. Local directory:
`/home/amit/Timepass Project/relational-db`. Module uses Go 1.25 and the standard
library; core database behavior is implemented from scratch.

Read AGENTS.md, progress.md, phase-1-plan.md, sql-to-storage-overview.md, and the
current stage design before work. Build top-down from SQL. Workflow: explain
concepts/options and a dedicated edge-case section -> discuss -> save agreed
design -> implement -> unit/integration tests -> record results -> commit/push
when requested. Each Go source/test file starts with 4–5 plain-language comment
lines. Use idiomatic Go/SOLID without speculative abstractions. No proactive
subagents. No Stage 5 implementation before its design discussion/agreement.

## Checkpoint

- Stages 1–3 implemented, verified, committed, and pushed: ab1b12f, b9bfd34,
  856f4bc respectively. main tracks origin/main.
- Stage 4 implemented and verified; included in the Stage 4 checkpoint commit.
- The user explicitly accepted literal-only SET, SQL NULL semantics, and preparing
  all memory changes before applying them, then requested Stage 4 implementation.

## Current behavior and limits

- CREATE TABLE; explicit-column single-row INSERT; SELECT * or named/repeated
  projections with optional WHERE; literal-only UPDATE; DELETE. Both mutations
  allow WHERE or all-row behavior without it.
- Types: strict int64 INTEGER (overflow rejected), UTF-8 TEXT, BOOLEAN. Every
  column nullable; omitted INSERT fields become typed NULL in schema order.
- Conditions: =, <>/!=, <, <=, >, >=; IS NULL / IS NOT NULL; AND/OR and parentheses.
  BOOLEAN comparison only equality/inequality. Same-type column-to-column and
  literal-to-column comparisons work. Bare BOOLEAN/TRUE/FALSE/NULL predicates work.
  WHERE rejects other types. AND binds tighter than OR; comparisons bind tighter.
- SQL three-valued logic: NULL comparisons yield unknown; only true matches WHERE.
  All branches analyzed before runtime short-circuiting. TEXT uses case-sensitive
  Go byte order without locale/normalization. Parentheses and tree depth each 256.
- UPDATE counts matched rows even if values were unchanged; DELETE counts removed
  rows. No matches succeeds with count zero. Duplicate SET targets are errors.
- Mutations scan original rows, prepare replacements, then publish through Memory.
  Preparation failures leave old memory rows intact. No transaction/crash guarantee.
- Independent sessions own schemas/rows; results and plans own copied mutable data.
  Exiting loses all state. No persistence/indexes/concurrency/server/permissions.
- Deferred SQL includes arithmetic/column-copy SET, unary NOT, joins, aliases,
  ORDER BY, LIKE/IN/BETWEEN, functions, RETURNING, comments, quoted identifiers.

## Code navigation and ownership

- internal/sql: tokenizer, AST, parser. expression.go owns expression structures and
  precedence parsing/depth checks. != normalizes to <> but retains original lexeme.
  Parser remains independent of schemas; errors have source positions.
- internal/types: immutable typed Value, int64 accessors, typed NULL, exact JSON.
- internal/catalog: ordered schemas, normalized names, defensive copies. Schema
  authority remains here; row-store collections do not establish table existence.
- internal/analyzer: bound CreateTable/Insert/Select/Update/Delete. Prepared Insert
  rows, Select positions/columns, resolved predicates, typed literal assignments.
  expression.go checks all branches/types and gives bare NULL its contextual type.
  Semantic errors preserve their type; ASTs lack semantic source spans.
- internal/expression: private bound nodes, Eval/Matches, three-valued logic,
  comparison without subtraction, and recursive Clone for ownership boundaries.
- internal/planner: small row plans, copied assignment/projection slices and cloned
  predicate trees. CREATE stays the session's catalog registration operation.
- internal/storage: zero-value Memory, table-name to row collections. Append owns
  input copies, Scan returns snapshots, Replace builds all owned copies before its
  final map assignment. Absent row collection means an empty catalog-resolved table.
- internal/executor: RowStore Append/Scan/Replace. SELECT filters original rows
  before projection. mutation.go prepares updates/survivors and publishes only
  after success; skips Replace for zero matches. Store failures retain causes and
  table context. Future backend failure semantics must be revisited in Stage 5.
- internal/engine: Session owns Catalog/Memory, Process parses, Apply analyzes then
  handles CREATE or plans/executes row commands. Result aliases executor.Result.
- internal/shell: one session per Run, JSON, semicolon-aware multiline framing.
  Completeness checked before Apply, SQL errors print/continue, I/O failures return.
- examples/stage-4.sql: current demonstration. scripts/verify_stage4.py: optional
  Python SQLite verification oracle; never used by engine execution.

## JSON result contract

- CREATE: kind create_table, status created-in-memory.
- INSERT: kind insert, status inserted, affected_rows 1.
- UPDATE/DELETE: kind update/delete, status updated/deleted, affected_rows count.
- AffectedRows is *int: non-nil for mutations including zero, nil/omitted otherwise.
- SELECT: kind select, status selected, query {columns, rows}. Columns have name/
  type; values have type/value. Empty rows are [] rather than null. Nonqueries
  omit query. Earlier command/prepared_row debugging output remains removed.

## Verification already passed

- gofmt, go test ./... -cover: SQL 99.7%, analyzer 96.3%, expression 94.4%, executor
  94.5%, engine 86.7%; planner/storage/shell/catalog/types 100%. cmd/db uses real
  executable checks instead of direct unit coverage.
- go vet ./...; go build ./...; build /tmp/relational-db-stage4 from cmd/db.
- python3 scripts/verify_stage4.py /tmp/relational-db-stage4: 16 Stage 4 example
  commands, 11 Stage 3 regression commands, and 341 deterministic SQL commands
  agree with SQLite (208 result multiset comparisons; 132 affected counts).
- Parser fuzz: go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s
  -parallel=2: 268,556 generated inputs, no failure.
- Tests cover all truth-table pairs, limits, precedence, NULL, strict analysis,
  unchanged rows after errors, duplicates/adjacent/all-row mutations, zero counts,
  original-row predicates, copy ownership, failing scan/evaluation/assignment/
  replace, incomplete shell commands, and independent tables/sessions.
- Go checks need build-cache access outside restricted paths; use normal approval
  if required. The optional Python check uses standard-library sqlite3 only.

## Next

Review Stage 4 and commit/push when requested. Explain and agree Stage 5 before
coding: persistent schemas/records, file/page layout, row IDs, allocation, variable
text, update/delete storage, size bounds, byte order/versioning, flush/close,
malformed-file errors, and clean close/reopen. Include edge cases and handling.
No existing file-format suggestion is binding. Persistence still targets clean
shutdown/reopen only; WAL/recovery/transactions are deferred.
