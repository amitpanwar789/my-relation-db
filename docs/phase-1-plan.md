# Phase 1: Embedded Relational Database

Status: agreed high-level plan; implementation has not started.

Date: 2026-10-29

## Purpose

Build a relational database from scratch in Go to learn how database internals
work: how rows are stored on disk, how indexes locate records, and how SQL becomes
executable operations. Treat correctness, clear design, verification, and
documentation as professional engineering requirements. Feature breadth and a
fixed delivery date are secondary to understanding the implementation.

SQLite and PostgreSQL may serve as design references. This project is an
independent engine and does not promise compatibility with either system's file
format, wire protocol, or full SQL dialect. Later phases may progressively add
capabilities associated with larger relational databases.

## Agreed scope

- Language: Go.
- Embedded engine with an interactive command-line SQL shell.
- Persistent tables and schema metadata on local disk.
- SQL subset: CREATE TABLE, INSERT, SELECT, UPDATE, DELETE, and CREATE INDEX.
- Types: INTEGER, TEXT, and BOOLEAN, with explicitly defined NULL behavior.
- Single-table queries, selected columns, comparisons, and WHERE predicates.
- AND/OR expressions, with precedence and NULL semantics defined before coding.
- Table scans and persistent single-column indexes.
- A basic EXPLAIN facility showing the chosen access method.
- One process using a database, executing commands sequentially.

These are capability targets, not a complete SQL grammar. Exact syntax, type
limits, operators, and error behavior will be specified in the relevant stages.
Unsupported SQL must return clear errors.

## Deferred scope

- Joins, aggregation, subqueries, and advanced query optimization.
- Transactions, rollback, write-ahead logging, and crash recovery.
- Concurrent clients, concurrent writers, locking, and row versioning/MVCC.
- Database server, network protocol, authentication, and permissions.
- Replication, distributed operation, and PostgreSQL compatibility.
- Additional SQL features, constraints, and data types unless explicitly added
  through a scope decision.

## Persistence and failure boundary

Phase 1 targets persistence after a successful clean shutdown and reopening.
It does not promise atomic statements, ACID transactions, or durability across
process crashes, operating-system crashes, or power loss. Interrupted writes may
leave partial changes or inconsistent tables and indexes. Error handling must
report failures clearly without claiming rollback guarantees.

The storage interfaces should leave room for future recovery work. The exact
flush/close behavior and error-state handling will be specified during storage
design. Single-process use is an operating restriction; how to enforce or detect
multiple openers remains a design question.

## Implementation boundary

Implement the database's page storage, row encoding, schema catalog, index
structures, SQL processing, and query execution ourselves. Do not delegate core
database work to an existing database engine or persistent key-value store.
Standard-library utilities and supporting tools for tests, logging, and shell
interaction are acceptable. Any parser tooling will be an explicit stage design
decision.

## Stage sequence

Revised on 2026-10-29: the agreed learning and implementation order is top-down,
starting with SQL. This supersedes the original storage-first sequence. Read
[the SQL-to-storage walkthrough](sql-to-storage-overview.md) for the big picture.

Begin with in-memory schemas and rows, then add persistence beneath working SQL
operations. In-memory execution is an intentional milestone, not a persistence
promise. Design each stage before implementing it; detailed storage choices remain
open until Stage 5. Every implementation stage includes unit and integration tests.

### Stage 1: SQL input and parsing

Design the shell's input boundaries, supported grammar, tokens, AST, identifiers,
literals, and syntax errors. Build a shell, tokenizer, and parser. Statements can
be inspected as structured representations; they do not execute yet. Define which
syntax is introduced now and which syntax is added in later stages.

Verification: unit tests for tokens and ASTs, whitespace and quoted strings,
malformed/unsupported input, and useful error locations. Integration tests exercise
SQL input through the shell to the parser and its displayed output/errors.

### Stage 2: Catalog and semantic analysis

Design an in-memory schema catalog, table/column resolution, type rules, and
semantic error reporting. Execute CREATE TABLE against the catalog and validate
the other currently supported statements against it. Catalog execution is the
first narrow execution path; general row execution follows in Stage 3.

Verification: unit tests for catalog and validation rules; integration tests for
CREATE TABLE followed by valid and invalid statements, duplicate definitions,
unknown names, mismatched values, and NULL rules.

### Stage 3: Basic execution in memory

Design simple plans, row values, insertion, table scans, projection, and query
results. Keep the table access boundary small and driven by actual operations.
Execute CREATE TABLE, INSERT, SELECT *, and selected-column SELECT in one session.

Verification: unit tests for plan/executor behavior; integration tests through the
SQL pipeline for multiple tables, multiple inserts, projections, empty results,
and errors. Rows do not survive closing the engine yet.

### Stage 4: Expressions and mutations

Design WHERE, comparisons, AND/OR precedence, parentheses, NULL semantics, UPDATE,
and DELETE. Extend grammar and validation as needed. Execute basic SQL operations
against the in-memory table implementation.

Verification: expression unit tests and SQL integration tests for matching and
nonmatching rows, NULL, precedence, full-table mutations, repeated changes, and
unaffected rows. Specify and test the nontransactional mutation failure behavior.

### Stage 5: Persistent storage design and implementation

Explain storage requirements using the working SQL operations. Compare and decide
file organization, pages and allocation, headers/versioning, byte order, row
encoding, variable-length text, row identifiers, catalog persistence, free space,
record growth/deletion, size limits, and flush/close/error behavior. Implement the
storage dependencies in practical increments, then connect them beneath the engine.
No earlier storage suggestion is binding without this design discussion.

Verification: unit tests for encodings, pages, record boundaries, and error paths;
integration tests for the SQL pipeline against disk storage, including clean
shutdown/reopen, persistent schemas, multi-page tables, updates/deletes, and
malformed/truncated files where supported. No crash recovery guarantee is added.

### Stage 6: Persistent indexes

Design disk-backed B+ trees, key ordering/encoding, duplicate and NULL handling,
indexed types/size limits, page splits/root changes, metadata, building indexes
on existing data, deletion strategy, and maintenance during successful mutations.
Implement CREATE INDEX and internal persistent index lookups.

Verification: tree unit tests and integration tests for index creation, reopening,
INSERT/UPDATE/DELETE maintenance, duplicate keys, multi-level splits, and lookup
agreement with scans. Failed builds must not appear as usable indexes.

### Stage 7: Index-aware planning

Design rules for choosing scans versus supported index access, remaining predicate
evaluation, and basic EXPLAIN output. Extend the simple planner introduced earlier;
a sophisticated cost-based optimizer is deferred.

Verification: planner unit tests and integration tests proving indexed and scan
execution produce identical results before and after mutations, including duplicate
and NULL cases. Check plans and targeted measurements to confirm actual index use.

### Stage 8: Release verification

Complete broader end-to-end/randomized checks, malformed-file handling, SQL and
file-format references, examples, known limitations, reproducible measurements,
and a repeatable build/check process. Test throughout all stages; this stage
broadens verification. Publication remains a separate decision.

Verification: run documented checks from a clean checkout and the full completion
demonstration below. Corruption detection does not imply recovery support.

## Working method for every stage

1. Explain the problem with concrete examples.
2. Compare relevant designs and their consequences.
3. Agree and record a stage design, invariants, limitations, and acceptance checks.
4. Implement small, reviewable increments.
5. Run meaningful verification and inspect the behavior together.
6. Record the outcome before proceeding to the next stage.

Stage design documents will live alongside this plan, for example
`docs/stage-1-sql-parsing-design.md`. No detailed design is implied by an example
filename. Maintain tests throughout implementation; stage 8 broadens verification
rather than introducing it. Record scope changes explicitly.

## Phase 1 completion demonstration

- Open a database through the shell and create a table with the supported types.
- Insert rows containing representative values, including NULL and text boundaries.
- Query all rows, project selected columns, and filter using documented predicates.
- Update and delete matching rows and check unaffected rows.
- Create an index over existing data and perform subsequent mutations.
- Show an eligible indexed query with EXPLAIN and compare its results with a scan.
- Close cleanly, reopen, and verify schemas, rows, indexes, and query results.
- Demonstrate useful errors for invalid SQL and unsupported operations.
- Publish documentation of the supported subset and clean-shutdown-only guarantees
  within the repository; public release remains a separate decision.

## Decisions intentionally left open

Page size, file count, physical row layout, catalog layout, caching strategy,
integer width/overflow behavior, text encoding and comparison, identifier casing,
maximum row/key sizes, exact index deletion strategy, and detailed package
boundaries will be decided during their stages. Avoid implementing speculative
infrastructure for later phases.

## Next discussion

Stage 1: discuss SQL input, the tokenizer, parser, AST, and syntax errors; agree on
a bounded grammar and acceptance tests before writing engine code.
