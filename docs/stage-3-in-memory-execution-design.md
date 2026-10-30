# Stage 3: Basic Execution in Memory

Status: proposed design for discussion, not implemented or finalized.

Date: 2026-10-29

## Current state and prerequisite

Stage 1 (SQL input and parsing) is implemented and tested. Stage 2 (catalog and
semantic analysis) is now implemented and verified (2026-10-30). This document previews Stage 3 at the
user's request; it does not change stage numbering or mark Stage 2 complete.
Before Stage 3 execution code, reconcile this draft with the implemented Stage 2
types and contracts and discuss the execution design with the user.

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

## Proposed responsibilities

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

These are responsibilities, not a requirement for one package per box. Select
concrete Go package boundaries after Stage 2 exists and avoid cyclic dependencies.
Use a narrow table-access interface at the executor boundary if it provides a
real seam for memory-backed storage now and persistent storage in Stage 5; do not
invent unused methods for future updates, transactions, or indexes.

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

1. Receive a semantically validated statement with resolved column positions.
2. Build the complete row in schema order using Stage 2's missing-value policy.
3. Append an owned copy to the correct table's in-memory rows.
4. Return an acknowledgement with one affected row.

Rejected commands must not append a partial row. This local validation property
does not establish general transactional or crash-recovery guarantees.

## SELECT behavior

1. Resolve output positions in requested order; expand SELECT * in schema order.
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
and display. Exact JSON/table output formatting is a reviewable choice before
implementation; the engine's result structure must not depend on that choice.

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

- Plan generation: INSERT remapping, SELECT projection positions, wildcard expansion.
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

## Open decisions and implementation status

Finalize after Stage 2: bound statement/result contracts, package boundaries,
integer and NULL policies, missing columns, and shell result presentation.
No code or tests for Stage 3 have been written, and no checks are claimed for it.
The next step is the Stage 3 design discussion using Stage 2's implemented
contracts; this draft is not yet an agreed Stage 3 design.
