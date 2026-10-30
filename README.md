# Relational DB

A relational database engine built from scratch in Go to understand storage,
indexes, and SQL execution through a carefully tested implementation.

The first phase targets an embedded engine with a SQL shell, persistent tables,
single-table queries, and indexes. Stages 1–4 are implemented: the shell parses
and validates SQL, creates tables, inserts rows, filters queries, and updates or
deletes matching rows in memory. There are no database files yet; exiting loses all tables and rows.

Start with the [SQL-to-storage walkthrough](docs/sql-to-storage-overview.md), then
read the [Phase 1 high-level plan](docs/phase-1-plan.md). We build from SQL downward,
using in-memory execution before adding persistence. Detailed stage designs will
be discussed and recorded before their implementation.

[AGENTS.md](AGENTS.md) records the project instructions for Codex, including clean
Go code, SOLID principles, and unit and integration tests in every implementation
stage.

## Run the current engine (Stage 4)

Requires Go 1.25 or later. No third-party dependencies are used.

```sh
go run ./cmd/db
```

Enter a statement ending in a semicolon. Multiline input is supported; EOF
(Ctrl-D on an empty terminal line) exits. Piped input omits prompts:

```sh
go run ./cmd/db < examples/stage-4.sql
```

Supported forms:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (id, name, active) VALUES (42, 'Amit', TRUE);
SELECT * FROM users;
SELECT id, name FROM users;
SELECT name FROM users WHERE id >= 10 AND active = TRUE;
UPDATE users SET name = 'Updated', active = FALSE WHERE id = 42;
DELETE FROM users WHERE active IS NULL;
```

Keywords are case-insensitive; ASCII unquoted identifiers normalize to lowercase.
Strings support UTF-8, embedded semicolons, and doubled quotes (`'O''Brien'`).
INSERT accepts one row with an explicit column list; literals include signed
64-bit integers, text, TRUE/FALSE, and NULL. The analyzer rejects unknown tables,
unknown columns, duplicate definitions/insert targets, wrong value counts, type
mismatches, and integer overflow. Values are not implicitly converted between types.
All columns are nullable; omitted INSERT columns receive NULL. Reordered INSERT
columns are mapped into schema order. Repeated SELECT columns are allowed, and
SELECT * follows schema order.

CREATE TABLE reports `created-in-memory`. INSERT reports `inserted` with
`affected_rows: 1`. SELECT reports `selected` with a `query` object containing
`columns` (names/types) and `rows` (typed values). For example:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (name, id) VALUES ('Amit', 42);
SELECT name, id FROM users;
```

The INSERT stores [42, 'Amit', NULL] in table column order. The SELECT returns
['Amit', 42]. Each value is encoded as `{"type":"TEXT","value":"Amit"}` or
its INTEGER/BOOLEAN equivalent; NULL uses a JSON null value with the column's type.
An empty SELECT includes its columns and `rows: []`. Query results are collected
in memory and copied so callers cannot change stored rows through returned slices.
All queries currently scan the table. SQL row order is unspecified without ORDER BY,
which is not supported yet. Each shell run has its own catalog and row store.
The parser remains independently testable and still accepts unknown names; the
session's semantic analyzer is responsible for rejecting them.

WHERE supports =, <> (or !=), <, <=, >, >=, AND/OR, parentheses, IS NULL, and
IS NOT NULL. Comparisons/NULL tests bind before AND, which binds before OR.
Operands can be column references or literals. Types must match; BOOLEAN allows
only equality/inequality. Bare BOOLEAN columns and TRUE/FALSE/NULL are valid
conditions. Text comparisons use case-sensitive Go string byte order, without
locale rules or Unicode normalization. Parenthesis nesting and expression-tree
depth are each limited to 256.

NULL comparisons produce unknown; WHERE keeps only true rows. Use IS NULL to
find missing values. UPDATE supports literal-only SET assignments, with duplicate
targets rejected. UPDATE/DELETE without WHERE affect all rows. They report
`updated`/`deleted` with `affected_rows`, including zero. UPDATE counts matches,
even when assigned values were unchanged. Mutations build replacement rows before
publishing them, so preparation errors leave existing memory rows unchanged.
This is not transaction support or a persistence/crash guarantee.

Indexes, persistence, comments, quoted identifiers, constraints, joins, arithmetic,
general unary NOT, and ORDER BY are not implemented. Syntax errors include line/
column positions; semantic errors identify affected names/types. The shell continues
with the next statement. An unmatched quote consumes the remaining input. Nonblank
SQL at EOF without a semicolon is reported as incomplete. Statement errors are
printed to output; stream failures produce a nonzero exit.

## Verify

```sh
go test ./...
go vet ./...
go build ./...
go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2
```

Unit tests cover tokenization, ASTs, typed values, schemas, semantic validation,
plans, execution, and row-store ownership. Session and shell integration tests
cover actual rows/projections, table/session isolation, invalid commands leaving
rows intact, typed boundaries/NULL, multiline/quoted input, recovery, and stream errors.
Stage 4 adds three-valued truth tables, precedence, duplicate/all-row mutations,
zero counts, predicate ownership, and injected preparation/publication failures.

For a repeatable real-shell comparison against Python’s standard-library SQLite
(optional Python 3 with sqlite3; no engine dependency):

```sh
go build -o /tmp/relational-db-stage4 ./cmd/db
python3 scripts/verify_stage4.py /tmp/relational-db-stage4
```

## Design discussions

- [SQL-to-storage big picture](docs/sql-to-storage-overview.md)
- [Phase 1 plan and stage sequence](docs/phase-1-plan.md)
- [Stage 1 design, examples, and verification](docs/stage-1-sql-parsing-design.md)
- [Stage 2 design and verification: catalog and semantic analysis](docs/stage-2-catalog-semantic-design.md)
- [Current progress and next action](docs/progress.md)
- [Session handoff and current checkpoint](docs/session-handoff.md)
- [Stage 3 design and verification: execution in memory](docs/stage-3-in-memory-execution-design.md)
- [Stage 4 design, edge cases, and verification: expressions and mutations](docs/stage-4-expressions-mutations-design.md)
