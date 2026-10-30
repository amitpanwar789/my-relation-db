# Stage 3: Basic Execution in Memory

Status: design accepted, implemented, and verified on 2026-10-30.

Updated: 2026-10-30

## Current state and prerequisite

Stage 1 (SQL input and parsing) is implemented and tested. Stage 2 (catalog and
semantic analysis) is implemented and verified. This discussion is reconciled
with Stage 2's actual types and commands. The user accepted the design below
with the instruction to implement Stage 3.

## Goal and explanation

Stage 1 understands a command's structure. Stage 2 checks names and types against
schemas. Stage 3 performs validated INSERT and SELECT commands using rows kept in
Go memory. CREATE TABLE uses the catalog execution path introduced in Stage 2.
No database files are involved; ending a session loses its tables and rows.

Example session:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (name, active, id) VALUES ('Amit', TRUE, 42);
SELECT id, name FROM users;
```

Expected logical query result: columns [id, name], one row [42, 'Amit'].
The different INSERT column order is deliberate: we must map input values to
schema positions rather than store them in the order the user happened to write.
Stage 2 already performs that mapping: its bound Insert.Row is
`[42, 'Amit', TRUE]`. Stage 3 must reuse that prepared row, not remap it again.

The pipeline becomes:

```text
Shell -> Parser -> Semantic analysis -> Simple planner -> Executor
                                                        |
                                                        v
                                           In-memory tables and rows
                                                        |
                                                        v
                                           Result -> Shell formatting
```

## Component responsibilities

- Engine/session: own one catalog and one set of in-memory tables for a session;
  coordinate parsing, validation, planning, and execution. Do not recreate this
  state for each shell statement. Separate sessions must not share mutable state.
- Catalog/analyzer (Stage 2): own schemas and validation rules, resolve table and
  column references, and return validated/bound information. Do not duplicate
  these checks in the planner or introduce a second schema authority.
- Planner: turn validated commands into a small explicit operation description.
  INSERT contains values in schema order. SELECT contains the target table and
  ordered output column positions. Start with one scan strategy; no optimizer or
  general operator framework is necessary.
- Executor: carry out a plan, append inserted rows, or scan/project selected rows.
  Return results and errors without printing, reading terminal input, or opening
  files.
- In-memory table store: manage rows belonging to each table and provide the small
  insertion/scan operations actually needed. Keep schemas in catalog metadata.
- Shell/result formatting: display command acknowledgements or query columns/rows.
  Keep formatting outside execution. Preserve useful error messages and continue
  after a rejected command as in Stage 1.

The implementation adds internal/planner, internal/executor, and internal/storage.
Dependencies remain acyclic. The executor's RowStore interface contains only
Append and Scan, providing the current memory-store boundary and a place for
persistent storage in Stage 5 without unused future operations.

## Walkthrough: from a bound command to a result

For the example INSERT, analysis has already supplied the table schema and the
typed row `[42, 'Amit', TRUE]`. The planner describes an InsertRows operation for
users. The executor asks the row store to append a copy and returns one affected
row. Planning alone must never change stored data.

For `SELECT name, id FROM users`, analysis supplies positions `[1, 0]` and output
column metadata. The planner describes a Project operation over a TableScan of
users. A full scan visits each stored row; projection constructs an output row
using those positions. `[42, 'Amit', TRUE]` becomes `['Amit', 42]`.

The planner answers "which operations?"; the executor performs them. There is
only one access method now. A small scan-and-projection plan keeps that distinction
visible without a general optimizer or an elaborate tree of operator interfaces.

## Accepted design choices

- Keep rows as `[]types.Value` in schema order, with a slice of rows per table.
  Named maps are readable but duplicate schema names and do not naturally express
  column order. Encoded bytes introduce persistence concerns too early.
- Reuse Stage 2's signed int64 INTEGER, TEXT, BOOLEAN, strict typing, and typed
  NULL. All columns remain nullable; omitted INSERT columns are already filled
  with NULL by analysis. Repeated SELECT columns remain valid.
- Introduce small planner, executor, and memory-store components. Keep the engine
  as coordinator, catalog as schema authority, and shell as presentation. Bound
  commands could execute directly today, but explicit small plans make the
  learning pipeline visible and provide a place for later access-method choices.
- Use a materialized query result: collect output rows before returning. It is
  simpler to own and test than a streaming iterator. Its memory cost grows with
  the result; revisit streaming when workloads justify it.
- Keep JSON shell output for this stage. Return command kind/status and affected
  row count for mutations, or column metadata and typed rows for queries. Empty
  SELECT results still include columns and an empty row list. Replace the
  validation-only command dump with execution results; update shell tests.
- Make an existing catalog table with no row-store entry mean an empty table.
  INSERT lazily allocates its row collection. CREATE then needs only the existing
  catalog registration path, avoiding two separately fallible registrations.
  An absent row collection does not establish that a SQL table exists: the
  analyzer must first resolve it in the catalog.
- Copy inserted rows and returned row slices. Values themselves are immutable.
  A scan may initially return a snapshot; this spends memory for simple ownership
  and can be revised behind the table-access boundary later.

Conceptual table-access operations are append-row and scan-rows. Add no update,
delete, page, index, or transaction methods yet. Keep any interface small and
defined by the executor's current needs. The memory store's table-name lookup is
only for locating row collections; it is not an index over SQL column values.

## Values and row representation

Reuse or extend Stage 2's typed values: INTEGER, TEXT, BOOLEAN, and explicit NULL.
Do not use arbitrary untyped maps or the parser's literal spelling as the execution
value model. Integer range, conversion, type compatibility, missing-column policy,
and nullability must come from the agreed Stage 2 contract, not an assumption here.

A row can initially be a slice of typed values in schema order. A table can hold
a slice of rows. This is readable, supports straightforward full scans, and avoids
introducing disk layouts before persistence is needed. It is deliberately not a
storage format to serialize directly to disk later.

## INSERT behavior

1. Receive a semantically validated statement containing its prepared typed row.
2. Plan insertion using that row; analysis already applied schema ordering and
   the missing-value policy.
3. Append an owned copy to the correct table's in-memory rows.
4. Return an acknowledgement with one affected row.

Rejected commands must not append a partial row. This local validation property
does not establish general transactional or crash-recovery guarantees.

## SELECT behavior

1. Use analysis's output positions and columns; wildcard expansion and name
   resolution already happened in Stage 2.
2. Scan the table's in-memory rows.
3. Project only those positions into each output row.
4. Return named columns and typed rows, including columns when the table is empty.

Example: a stored row [42, 'Amit', TRUE] queried by SELECT name, id FROM users
produces ['Amit', 42]. Preserve repeated projection columns if permitted by Stage 2.
Do not promise SQL ordering without ORDER BY even if the current slice scan
naturally visits rows in insertion order. Verify multi-row results without relying
on an undocumented SQL ordering guarantee.

## Results and state ownership

Represent command acknowledgements and row results explicitly. Query results
include column names/types and rows; do not expose mutable table-owned slices.
Copy slices at ownership boundaries so changing a returned result cannot change
stored data. Keep NULL distinct from empty text, zero, and FALSE in both execution
and display. The engine returns structured results; the shell serializes them
as JSON. Execution has no terminal-formatting logic.

The result contract is:

- CREATE: kind `create_table`, status `created-in-memory`.
- INSERT: kind `insert`, status `inserted`, `affected_rows: 1`.
- SELECT: kind `select`, status `selected`, a `query` object containing `columns`
  and `rows`. Each column has a name/type; each value has a type/value. Empty
  tables return `rows: []`. Non-query results omit `query`.

Stage 2's `command`/`prepared_row` validation dumps are replaced by execution
results. This is an intentional shell output change, not a SQL grammar change.

## Scope, errors, and invariants

- Execute CREATE TABLE, INSERT, SELECT *, and named-column SELECT in one session.
- No WHERE, joins, UPDATE/DELETE, indexes, disk persistence, concurrency, or server.
- Keep unknown-table/column, duplicate-definition, value-count, and type errors
  in the Stage 2 validation contract. Report execution/store errors with context.
- SELECT must not mutate stored rows. Tables and independent sessions stay isolated.
- Copy input rows/results where needed to avoid accidental shared mutable storage.
- Use the agreed Stage 2 CREATE TABLE lifecycle so failed definitions do not leave
  catalog and row-store state disagreeing.
- Closing the engine discards all state; reopening creates an empty session.

## Tests and acceptance criteria

Unit tests:

- Plan generation: preserve analysis's prepared INSERT row, projection positions,
  and wildcard-expanded metadata; planning does not mutate state.
- Executor/store: insertion, full scans, empty tables, typed values and NULL.
- Ownership: changing submitted row slices or returned results cannot mutate storage.
- Error paths and independent table/session state.

Integration tests:

- CREATE TABLE -> INSERT -> SELECT through the full SQL pipeline in one session.
- Multiple rows/tables, reordered INSERT columns, reordered/repeated projections
  where allowed, and all supported values at Stage 2's defined boundaries.
- Invalid SQL/semantic failures followed by valid commands; rejected commands do
  not change previously stored rows.
- Closing/reopening loses all state, as documented.
- Shell displays query results and acknowledgements, not only ASTs; EOF and stream
  errors retain their Stage 1 behavior.

Before completion, format Go files and run `go test ./...`, `go vet ./...`, and
`go build ./...`, then exercise a real example SQL session. Record actual commands
and outcomes here. All new source/test files require the explanatory headers
specified in AGENTS.md.

## Implementation and verification

- internal/planner/plan.go builds Insert and Select plans; Select contains a
  TableScan and resolved projection positions/metadata. Planning copies slices.
- internal/storage/memory.go owns table row collections. Zero-value Memory works;
  Append copies a row, and Scan returns a copied snapshot. No catalog duplication.
- internal/executor/executor.go executes plans through RowStore and returns Result
  and QueryResult structures. Store errors preserve their causes and gain table
  context; invalid projection positions report errors rather than panic.
- internal/engine/session.go owns the catalog and memory store. CREATE uses catalog
  registration; all row operations pass through planning/execution. Shell stream
  framing and error recovery are retained.
- Unit tests cover plan preservation/ownership, store snapshots/isolation,
  execution/projection, and failing storage. Engine and shell integration tests
  cover SQL execution, boundaries/NULL, independent state, rejected commands,
  result mutation, exact JSON integers, Unicode, prompts, and stream failures.
- examples/stage-3.sql demonstrates empty results, multiple rows/tables, reordered
  inserts, repeated projection, NULL, booleans, and integer boundaries.

Verification passed on 2026-10-30:

- gofmt on changed Go files; `git diff --check`.
- `go test ./... -cover`: planner, executor, storage, shell, catalog, and types
  100%; analyzer 94.5%; SQL 99.5%; engine 86.7%. The command entry point has no
  direct unit coverage and was checked using the built executable.
- `go vet ./...`; `go build ./...`.
- `go build -o /tmp/relational-db-stage3 ./cmd/db` followed by a real-process smoke
  check of all 11 commands in examples/stage-3.sql. Checked returned rows, exact
  min/max int64, Unicode/quotes/semicolons, NULL, repeated projection, empty results,
  and five successful inserts across two tables.
- Real-process rejection/recovery checks: five semantic failures left prior rows
  intact, a later insert succeeded, a fresh process had no prior tables, and an
  INSERT missing its final semicolon was rejected before execution.

Materialized scan snapshots and query results use memory proportional to data
size. There is no WHERE, UPDATE/DELETE, persistence, index access, concurrency,
transaction support, or crash guarantee. Stage 4 begins with a design discussion.
