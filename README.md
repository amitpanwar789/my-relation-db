# Relational DB

A relational database engine built from scratch in Go to understand storage,
indexes, and SQL execution through a carefully tested implementation.

The first phase targets an embedded engine with a SQL shell, persistent tables,
single-table queries, and indexes. Stages 1 and 2 are implemented: the shell parses
SQL, creates table schemas in memory, and validates INSERT/SELECT against those
schemas. It does not store rows, retrieve query results, or access database files yet.

Start with the [SQL-to-storage walkthrough](docs/sql-to-storage-overview.md), then
read the [Phase 1 high-level plan](docs/phase-1-plan.md). We build from SQL downward,
using in-memory execution before adding persistence. Detailed stage designs will
be discussed and recorded before their implementation.

[AGENTS.md](AGENTS.md) records the project instructions for Codex, including clean
Go code, SOLID principles, and unit and integration tests in every implementation
stage.

## Run the current engine (Stage 2)

Requires Go 1.25 or later. No third-party dependencies are used.

```sh
go run ./cmd/db
```

Enter a statement ending in a semicolon. Multiline input is supported; EOF
(Ctrl-D on an empty terminal line) exits. Piped input omits prompts:

```sh
go run ./cmd/db < examples/stage-2.sql
```

Supported forms:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (id, name, active) VALUES (42, 'Amit', TRUE);
SELECT * FROM users;
SELECT id, name FROM users;
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

CREATE TABLE reports `created-in-memory`. INSERT and SELECT report `validated-only`
and display the resolved command, not stored data or query rows. For example:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (name, id) VALUES ('Amit', 42);
```

The second command shows prepared values [42, 'Amit', NULL] in table column order,
but does not insert them. Each shell run has its own catalog; exiting loses schemas.
The parser remains independently testable and still accepts unknown names; the
session's semantic analyzer is responsible for rejecting them.

WHERE, UPDATE/DELETE, indexes, comments, quoted identifiers, constraints, joins,
and row execution are not implemented. Syntax errors include line/column positions;
semantic errors identify affected names/types. The shell continues with the next
statement. An unmatched quote consumes the
remaining input. Nonblank SQL at EOF without a semicolon is reported as incomplete.
Statement errors are printed to output; stream failures produce a nonzero exit.

## Verify

```sh
go test ./...
go vet ./...
go build ./...
go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2
```

Unit tests cover tokenization, ASTs, typed values, schema ownership, and semantic
validation. Session and shell integration tests cover catalog lifetime, invalid
commands, input-to-result output, multiline/quoted input, recovery, and stream errors.

## Design discussions

- [SQL-to-storage big picture](docs/sql-to-storage-overview.md)
- [Phase 1 plan and stage sequence](docs/phase-1-plan.md)
- [Stage 1 design, examples, and verification](docs/stage-1-sql-parsing-design.md)
- [Stage 2 design and verification: catalog and semantic analysis](docs/stage-2-catalog-semantic-design.md)
- [Current progress and next action](docs/progress.md)
- [Session handoff for resuming at Stage 3](docs/session-handoff.md)
- [Stage 3 proposed design: execution in memory](docs/stage-3-in-memory-execution-design.md)
  (discussion draft; reconcile with the implemented Stage 2 before Stage 3 coding)
