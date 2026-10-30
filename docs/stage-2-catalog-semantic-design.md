# Stage 2: Catalog and Semantic Analysis

Status: design accepted and implementation verified on 2026-10-30.

Date: 2026-10-29

## Goal and explanation

Stage 1 checks whether SQL is structurally understandable. Stage 2 checks whether
the command makes sense against known table definitions. It also executes a narrow
operation: CREATE TABLE registers a schema in memory. INSERT and SELECT are checked
and resolved, but do not insert or retrieve rows until Stage 3.

```text
SQL -> existing tokenizer/parser -> AST -> semantic analyzer -> bound command
                                               ^                   |
                                               |                   v
                                        in-memory catalog     session result
```

A session owns the catalog and retains it between statements. Restarting the
application creates an empty catalog. This is not disk persistence.

## Example walkthrough

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (name, id) VALUES ('Amit', 42);
SELECT name, id FROM users;
```

1. Validate the proposed table: no existing users table, no duplicate column names,
   and only supported types. Register it only after all checks succeed.
2. Store ordered metadata: id at position 0, name at 1, active at 2.
3. Validate INSERT: users exists; both target columns exist and are not repeated;
   two values match two input columns; the literals match their target types.
4. Convert literals to typed values and map them to schema order. Under the
   agreed nullable/missing-value policy the prepared row is [42, 'Amit', NULL].
   Do not store this row in Stage 2.
5. Validate SELECT: both columns exist and resolve to positions [1, 0]. Keep their
   requested order. No row scan happens yet.

Examples of syntactically valid but semantically rejected SQL after that CREATE:

```sql
SELECT missing FROM users;
SELECT id FROM unknown_table;
INSERT INTO users (id) VALUES ('forty-two');
INSERT INTO users (id, id) VALUES (1, 2);
INSERT INTO users (id, name) VALUES (1);
CREATE TABLE users (other INTEGER);
CREATE TABLE other (id INTEGER, id TEXT);
```

## Agreed policies

- INTEGER is a signed 64-bit value, from -9223372036854775808 through
  9223372036854775807. Convert explicitly and reject overflow; the AST remains
  lossless text and does not perform this range check.
- TEXT is the decoded valid UTF-8 string from parsing. No new text-size limit or
  collation decision is needed here; persistent record limits are a later design.
- BOOLEAN accepts only TRUE/FALSE literals. Do not silently coerce integers or text.
- NULL is a distinct typed-value state, not empty text, zero, or FALSE. All columns
  are nullable initially because NOT NULL constraints are outside the grammar.
- Omitted INSERT columns receive NULL. Explicit values must match the explicitly
  named columns in count and type. INSERT already requires an explicit column list.
- Reordered INSERT columns resolve to schema order before future execution.
- Duplicate table definitions, duplicate CREATE columns, and duplicate INSERT
  target columns are errors. Names follow Stage 1's lowercase normalization.
- Repeated SELECT projections are allowed: SELECT name, name returns positions
  [1, 1]. SELECT * expands in schema-definition order.
- Rejected statements do not change catalog state. This validation guarantee does
  not claim transactions, statement rollback for future writes, or crash recovery.

Missing columns could instead be rejected, and integer width could differ. The
recommendation above makes behavior explicit without adding constraints/defaults
machinery. The user authorized implementation of these proposed policies on 2026-10-30.

## Component responsibilities

- Shared type/value definitions: INTEGER/TEXT/BOOLEAN and explicit NULL, using
  concrete Go values (int64, string, bool) rather than parser literal spelling.
  Keep these independent of shell, parser behavior, and storage.
- Catalog: own table schemas and ordered columns; register and look up tables.
  Do not store rows. Protect owned metadata from modification through returned
  slices or caller-provided schema objects.
- Semantic analyzer: accept an AST plus schema lookup, resolve names to column
  positions, check types/counts, convert literals, and return a bound command.
  Analyze without changing the catalog. Semantic errors name the table/column and
  expected/actual type as relevant.
- Session/engine: own one catalog, coordinate parsing and analysis, and apply
  validated CREATE TABLE commands. Reuse the same session throughout a shell run.
  Keep independent sessions isolated. No general executor/planner is needed yet.
- Shell: keep existing framing and stream error behavior; show created versus
  validated-only results clearly, and continue after semantic errors.

Implemented package boundaries are internal/types, internal/catalog,
internal/analyzer, and internal/engine alongside the existing sql and shell
packages. Introduce only the small contracts actually needed. The analyzer can
consume a narrow catalog lookup interface; do not create interfaces for every
struct or add unused transaction/storage methods. Share type definitions rather
than maintaining contradictory parser/catalog/execution type lists.

## Bound commands and output

An AST records what was written. A bound command records what was resolved:

- CREATE TABLE: a validated schema awaiting registration.
- INSERT: target schema and concrete typed values in schema order.
- SELECT: target schema, ordered output positions, and output names/types.

These are bound commands for Stage 3 to consume, not query execution plans.
Do not put file offsets, pages, indexes, or terminal formatting into them.

Keep a structured shell result with an explicit status: CREATE TABLE reports
created-in-memory; INSERT/SELECT report validated-only. Successful validation must
never print "inserted" or return fictitious query rows. Continue using inspectable
JSON initially. Preserve exact int64 values in display (do not convert via float64).

Syntax errors retain Stage 1's positions. Current ASTs do not carry source spans;
semantic errors should identify the affected table/column without inventing a
line/column. Source spans can be added separately if needed.

## Scope and invariants

- No SQL grammar expansion, rows, scans, WHERE, UPDATE/DELETE, indexes, file I/O,
  networking, concurrency, or transactions.
- The parser remains usable without a catalog. Unknown names still parse there.
- Only validated CREATE TABLE changes session state. Validation of INSERT/SELECT
  reads metadata without mutating it.
- Schema order is authoritative; input/projection order can differ deliberately.
- Failed CREATE must not reserve a name or leave partial column metadata.
- Session ownership prevents accidental shared mutable schemas across callers.

## Tests and acceptance criteria

Unit tests:

- Catalog lookup/registration, duplicate names, ordered columns, defensive copying.
- Analyzer name resolution, wildcard expansion, reordered/repeated projections.
- INSERT remapping, missing columns, NULL, strict types, count errors, duplicate
  targets, all literal types, and int64 minimum/maximum/overflow.
- Failed CREATE does not affect state; separate sessions are independent.

Integration tests:

- SQL CREATE -> subsequent valid/invalid INSERT and SELECT in one session.
- Semantic error followed by a valid command; catalog remains usable.
- Invalid CREATE followed by a corrected CREATE with the same name succeeds.
- Closing/restarting loses schemas; output distinguishes created from validated.
- Retain Stage 1 unit tests and framing/error/stream behavior, adapting shell
  expectations to the new semantic boundary rather than weakening parser checks.

Acceptance: CREATE registers schemas, supported statements resolve against them,
invalid commands return useful errors, and no row execution/persistence is claimed.

Format Go files and run go test ./..., go vet ./..., and go build ./...; exercise
the real shell with the walkthrough and rejection examples. Record actual results
after implementation. Every source/test file retains its explanatory header.

## Implementation outcome

Implemented on 2026-10-30, without third-party dependencies. Core files:

- `internal/types/value.go`: shared type constants and immutable typed values;
  JSON preserves int64 values and distinct typed NULLs. AST types alias this shared
  definition while literal spellings remain lossless in the parser.
- `internal/catalog/catalog.go`: ordered schemas, normalized lookup, validation,
  and defensive copies on registration and lookup.
- `internal/analyzer/command.go` and `analyzer.go`: resolved CREATE/INSERT/SELECT
  commands, column positions, strict literal conversion, semantic diagnostics.
- `internal/engine/session.go`: one zero-value-usable session owns a catalog;
  Process parses and applies, while Apply accepts an already parsed statement.
  The shell uses Apply only after confirming a complete statement, preventing an
  unterminated CREATE from mutating session state.
- `internal/shell/shell.go`: one session per shell run; JSON results contain kind,
  status, and command. CREATE reports created-in-memory; INSERT/SELECT report
  validated-only. No row execution or disk persistence was added.

Unit tests cover typed values, catalog ownership, name resolution, NULL/missing
columns, int64 bounds/overflow, counts, strict types, duplicates, and projections.
Session/shell integration tests cover successful and rejected SQL, recovery,
independent session state, output snapshots, and incomplete input. Existing lexer
and parser tests remain in place; shell tests now exercise semantic validation.

Verification passed:

- `go test ./... -cover`: all packages passed. Analyzer 94.5%, catalog 100%,
  engine 92.3%, shell 100%, SQL 99.5%, types 100% statement coverage in this run.
  The small command entry point has no direct unit coverage and was smoke-tested.
- `go vet ./...`: no findings.
- `go build ./...` and `go build -o /tmp/relational-db-stage2 ./cmd/db`: passed.
- Real command with `examples/stage-2.sql`: verified statuses, typed/remapped/NULL
  values, exact maximum int64, Unicode/escaped quotes, wildcard/projection order.
- Real command rejection session: eight semantic rejection cases, recovery to a
  valid SELECT, a fresh session losing prior schemas, and incomplete CREATE input.

No Stage 3 row execution has been implemented. Stage 2 is included in the local
checkpoint commit along with docs/session-handoff.md; it has not been pushed.
