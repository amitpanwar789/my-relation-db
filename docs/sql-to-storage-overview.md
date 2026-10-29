# From SQL to Storage: The Phase 1 Big Picture

This walkthrough preserves the explanation agreed in our planning discussion.
Read it whenever you need to reconnect an individual component with the whole
database. The [Phase 1 plan](phase-1-plan.md) contains the current stage details.

## Why we start from SQL

Starting from SQL and following it down into storage fits how we want to learn.
The earlier sequence prioritized building dependencies first. Instead, we will
make each layer understandable and executable before introducing the layer beneath
it.

Let's follow one statement through the engine:

```sql
INSERT INTO users (id, name)
VALUES (42, 'Amit');
```

Conceptually, it travels through these layers:

```text
SQL text
   |
   v
Tokenizer              Recognize words, numbers, strings, and punctuation
   |
   v
Parser                 Understand the statement's grammatical structure
   |
   v
Semantic analysis      Resolve tables/columns and validate types
   |
   v
Planner                Choose the operations needed to execute it
   |
   v
Executor               Perform those operations
   |
   v
Table / record layer   Represent rows and locate their storage
   |
   v
Page-storage layer     Read and write blocks of bytes
   |
   v
Database file          Persist those bytes through the operating system
```

These are responsibilities. They don't necessarily need separate packages or
elaborate frameworks. The structures below are conceptual examples, not finalized
Go APIs or file formats.

## 1. Tokenizer: recognize the pieces

The input begins as an ordinary string. The tokenizer converts it into tokens:

```text
INSERT
INTO
identifier("users")
(
identifier("id")
,
identifier("name")
)
VALUES
(
integer(42)
,
string("Amit")
)
;
```

It recognizes that 'Amit' is a string literal and 42 is an integer literal. It
does not know whether users exists or whether inserting these values is valid.

## 2. Parser: understand the structure

The parser checks whether the tokens follow our SQL grammar and creates a
structured representation, usually called an abstract syntax tree, or AST.

Conceptually:

```text
InsertStatement
  table: users
  columns: [id, name]
  values: [IntegerLiteral(42), StringLiteral("Amit")]
```

This separates the meaning of the syntax from how the user formatted it.
Whitespace and keyword capitalization no longer matter once parsing is complete.

A missing closing parenthesis is a parser error. An unknown table is handled next.

## 3. Semantic analysis: check meaning against the database

Now the engine consults its catalog: the metadata describing tables and columns.

Suppose it contains:

```text
users
  id    INTEGER
  name  TEXT
```

This layer answers:

- Does users exist?
- Do id and name exist?
- Are columns repeated or required values missing?
- Are the supplied values compatible with their column types?

It can also resolve column names to internal positions or identifiers, so execution
doesn't repeatedly search by name.

Valid SQL syntax does not necessarily mean a valid database operation.

For CREATE TABLE, this layer instead checks the proposed schema, for example,
whether the table already exists. Required-value rules depend on the schema
features we actually choose to support.

## 4. Planner: decide what operations to perform

For an insert, the initial plan can be simple:

```text
Insert these validated values into users
```

For a query, the choices become more interesting:

```sql
SELECT name FROM users WHERE id = 42;
```

Without an index:

```text
Read every row in users
   |
   v
Keep rows where id = 42
   |
   v
Return only name
```

With a suitable index, we may later choose:

```text
Look up 42 in the id index
   |
   v
Fetch matching rows
   |
   v
Return only name
```

We'll begin with straightforward plans. A sophisticated optimizer is unnecessary
for our first engine.

## 5. Executor: perform the plan

The executor runs the chosen operations.

For the insert, it asks the table layer to insert the validated row. Later, if
indexes exist, the operation also needs to maintain those indexes.

For the select, it obtains rows, evaluates the predicate, and produces the
requested columns.

The executor should not need to know the byte offset of a row in a file. That
responsibility belongs below it.

## 6. Table and record layer: turn rows into stored records

This layer bridges logical rows and physical storage.

It handles questions such as:

- How do we encode an integer, text, or NULL?
- How do we find all records belonging to a table?
- How do we identify a record for update or deletion?
- Where can a new record fit?

For our insert, it eventually encodes:

```text
[42, "Amit"] -> record bytes
```

Then it places those bytes into the table's storage.

This is where page layouts, slots, and growing records become relevant. We'll
encounter those concepts with a clear reason for needing them.

## 7. Page-storage layer and file: persist the bytes

The page-storage layer manages fixed-size blocks of bytes. It provides operations
conceptually like:

```text
AllocatePage()
ReadPage(pageID)
WritePage(pageID, bytes)
```

The record layer uses these operations to read or change table pages.

The page-storage layer translates a page identifier into a file location and uses
the operating system's file APIs. The operating system handles the filesystem and
device beneath that.

A successful write call and data surviving a power failure are different
guarantees. We'll define our clean-shutdown persistence behavior when we reach
this layer; crash recovery remains outside Phase 1.

For a SELECT, the lower-level flow largely runs in reverse: read pages, decode
records, evaluate the query, and return values.

## How we build Phase 1 from the top down

We can build from the top down by initially keeping data in memory. That gives the
upper layers something real to operate on. Later, we replace the memory-backed
table implementation with persistent storage.

| Stage | Focus | Observable result |
| --- | --- | --- |
| 1. SQL input and parsing | Shell, tokenizer, parser, AST, syntax errors | Enter supported SQL and inspect its structured representation |
| 2. Catalog and semantic analysis | In-memory schemas, table/column resolution, type rules | Execute CREATE TABLE; validate other statements against known schemas |
| 3. Basic execution in memory | Simple plans, in-memory rows, INSERT and SELECT | Create a table, insert rows, and query selected columns |
| 4. Expressions and mutations | WHERE, comparisons, AND/OR, NULL, UPDATE, DELETE | The agreed basic SQL operations work during one session |
| 5. Persistent storage design and implementation | Record encoding, pages, files, allocation, persistent catalog | Tables and rows survive a clean shutdown and reopening |
| 6. Persistent indexes | B+ trees, CREATE INDEX, building and maintaining indexes | Index lookups work and remain consistent through mutations |
| 7. Index-aware planning | Choose scans or index access; basic EXPLAIN | SQL queries use suitable indexes and show the chosen plan |
| 8. Release verification | Broader correctness checks, errors, documentation, measurements | A documented and verified Phase 1 engine |

In Stage 5, we'll still implement the storage dependencies in a practical order,
but by then we'll understand which SQL operations need them and why.

We'll keep the early storage interface small and let working operations guide its
shape. We don't need to predict every future storage requirement before writing
our first query.

The first implementation milestone becomes: type an INSERT or SELECT statement
and see the engine correctly understand its structure. From there, we add meaning,
execution, persistence, and indexing in that order.
