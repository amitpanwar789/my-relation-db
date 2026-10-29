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

Stages build observable behavior incrementally. First establish working record
storage and scans; add indexes only after a baseline exists for comparison.
Detailed designs remain open until the corresponding stage discussion.

### Stage 1: Storage foundations

Discuss and decide:

- Database file organization and file header.
- Fixed-size pages, page identifiers, and page allocation.
- Binary encoding, byte order, format identification, and version handling.
- Basic file operations, flush/close semantics, and I/O errors.

Deliverable: a page-storage layer that can create a database, write pages, close
it, reopen it, and read the same pages.

Verification: page round trips across reopening, page boundaries, invalid page
references, truncated files, and supported error paths. Record the chosen format
and invariants.

### Stage 2: Tables and records

Discuss and decide:

- Persistent schema metadata and table discovery through a catalog.
- Typed row encoding, NULL representation, and variable-length text.
- Row identifiers and record placement inside pages.
- Page capacity, record-size limits, and growth across multiple pages.

Deliverable: internal Go APIs to create tables, insert typed records, and scan
tables, with schemas and data surviving reopening.

Verification: multiple tables and pages, type round trips, empty and boundary-size
text, NULL values, schema lookup after reopening, and invalid input handling.

### Stage 3: First SQL path

Discuss and decide:

- Tokenization, parsing, statement representation, and SQL grammar.
- Identifier rules, literals, name resolution, and type validation.
- A simple execution path and shell statement handling.

Deliverable: CREATE TABLE, INSERT, SELECT *, and selected-column SELECT through
the SQL shell, backed by persistent storage.

Verification: end-to-end SQL sessions, reopening, malformed SQL, unknown tables
and columns, invalid values, and unsupported syntax.

### Stage 4: Filtering and expressions

Discuss and decide:

- Column references, literals, and supported comparison operators.
- AND/OR, expression precedence, and parentheses.
- NULL semantics, including appropriate NULL predicates.
- Predicate evaluation over table scans.

Deliverable: single-table SELECT ... WHERE queries with documented expression
behavior.

Verification: expected result sets, operator precedence, NULL behavior, empty
results, and expression/type errors.

### Stage 5: Updates and deletes

Discuss and decide:

- UPDATE and DELETE syntax and predicate reuse.
- Replacing records when text grows or shrinks.
- Deleted slots, space reuse, and row-identifier stability.
- Mutation errors under the documented nontransactional failure model.

Deliverable: UPDATE and DELETE, with and without WHERE, whose successful changes
survive clean shutdown and reopening.

Verification: matching and nonmatching rows, full-table operations, repeated
mutations, growing text, space reuse, and unchanged neighboring records.

### Stage 6: Persistent indexes

Discuss and decide:

- Disk-backed B+ tree pages, key encoding, and ordering.
- Duplicate keys, NULL handling, supported indexed types, and key-size limits.
- Tree traversal, page splits, and root changes.
- Index metadata and building an index over existing rows.
- Maintenance during INSERT, UPDATE, and DELETE, including the deletion strategy.

Deliverable: CREATE INDEX for single-column indexes, persistent lookup through
an internal API, and consistent index maintenance during successful mutations.

Verification: duplicates, ordered/random insertion, multi-level splits, reopening,
updates/deletes, and agreement between index lookups and table scans. Failed or
interrupted index builds must not be presented as successfully usable indexes.

### Stage 7: Choosing how queries run

Discuss and decide:

- A simple plan representation and rules for choosing an access method.
- Predicates eligible for index use and remaining filter evaluation.
- The initial supported lookup/range cases and fallback behavior.
- Human-readable EXPLAIN output.

Deliverable: SQL queries use suitable indexes where supported, fall back to scans
otherwise, and expose the chosen access method through EXPLAIN.

Verification: identical results for indexed and scan execution before and after
mutations, including duplicate and NULL cases. Confirm actual index use through
plans and targeted measurements; a sophisticated cost-based optimizer is deferred.

### Stage 8: Release verification

Discuss and complete:

- Broader end-to-end and randomized verification.
- Detection and clear reporting of malformed or unsupported files where feasible.
- SQL reference, file-format documentation, usage examples, and known limits.
- Reproducible performance measurements with stated data sizes and environments.
- Formatting, static checks, automated tests, and a repeatable build process.

Deliverable: a documented first release with demonstrated behavior and explicit
failure guarantees. Publication is a later decision, not an automatic stage action.

Verification: run the agreed checks from a clean checkout and execute the complete
demonstration workflow below. Corruption detection does not imply recovery support.

## Working method for every stage

1. Explain the problem with concrete examples.
2. Compare relevant designs and their consequences.
3. Agree and record a stage design, invariants, limitations, and acceptance checks.
4. Implement small, reviewable increments.
5. Run meaningful verification and inspect the behavior together.
6. Record the outcome before proceeding to the next stage.

Stage design documents will live alongside this plan, for example
`docs/stage-1-storage-design.md`. No detailed design is implied by an example
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

Stage 1: explain pages and compare database file organizations, then agree on the
first storage design before writing engine code.
