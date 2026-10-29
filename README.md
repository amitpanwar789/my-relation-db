# Relational DB

A relational database engine built from scratch in Go to understand storage,
indexes, and SQL execution through a carefully tested implementation.

The first phase targets an embedded engine with a SQL shell, persistent tables,
single-table queries, and indexes. Stage 1 is implemented: the shell tokenizes and
parses a small SQL subset and displays its AST as JSON. It does not execute SQL,
create tables, store rows, or access database files yet.

Start with the [SQL-to-storage walkthrough](docs/sql-to-storage-overview.md), then
read the [Phase 1 high-level plan](docs/phase-1-plan.md). We build from SQL downward,
using in-memory execution before adding persistence. Detailed stage designs will
be discussed and recorded before their implementation.

[AGENTS.md](AGENTS.md) records the project instructions for Codex, including clean
Go code, SOLID principles, and unit and integration tests in every implementation
stage.

## Run Stage 1

Requires Go 1.25 or later. No third-party dependencies are used.

```sh
go run ./cmd/db
```

Enter a statement ending in a semicolon. Multiline input is supported; EOF
(Ctrl-D on an empty terminal line) exits. Piped input omits prompts:

```sh
go run ./cmd/db < examples/stage-1.sql
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
integers, text, TRUE/FALSE, and NULL. The parser deliberately accepts unknown
tables and semantic mismatches: schema validation is Stage 2.

WHERE, UPDATE/DELETE, indexes, comments, quoted identifiers, constraints, joins,
and SQL execution are not implemented. Errors include line/column positions and
the shell continues with the next statement. An unmatched quote consumes the
remaining input. Nonblank SQL at EOF without a semicolon is reported as incomplete.
Statement errors are printed to output; stream failures produce a nonzero exit.

## Verify

```sh
go test ./...
go vet ./...
go build ./...
go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2
```

Unit tests cover tokenization and ASTs. Shell integration tests cover the complete
input-to-JSON path, multiline/quoted input, diagnostics, recovery, and stream errors.

## Design discussions

- [SQL-to-storage big picture](docs/sql-to-storage-overview.md)
- [Phase 1 plan and stage sequence](docs/phase-1-plan.md)
- [Stage 1 design, examples, and verification](docs/stage-1-sql-parsing-design.md)
