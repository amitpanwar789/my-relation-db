# Stage 4: Expressions, Filtering, UPDATE, and DELETE

Status: agreed, implemented, and verified on 2026-10-30.

Updated: 2026-10-30

## Goal and prerequisites

Stages 1–3 are implemented and verified. Stage 3 is committed as 856f4bc and
pushed, including the previously local Stage 2 commit b9bfd34. Before this stage, SQL created tables, inserted rows, and scanned/projected
results in session memory. Stage 4 now adds filtering and mutations.

Stage 4 evaluates conditions on rows and uses them consistently for SELECT,
UPDATE, and DELETE. All data remains in memory. No persistence, indexes,
transactions, concurrency, joins, arithmetic, or general optimizer is introduced.
This document records the agreed design and the implemented behavior below.

## Examples and observable behavior

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (id, name, active) VALUES (1, 'Amit', TRUE);
INSERT INTO users (id, name, active) VALUES (2, 'Neha', FALSE);
INSERT INTO users (id, name) VALUES (3, 'Ravi');

SELECT name FROM users WHERE id >= 2 AND active = FALSE;
UPDATE users SET active = TRUE, name = 'Updated' WHERE id = 2;
DELETE FROM users WHERE active IS NULL;
```

The SELECT returns Neha. UPDATE matches one row and changes only its named
columns. DELETE removes Ravi's row, whose omitted active value is NULL. UPDATE
and DELETE without WHERE affect all rows; no matches is a successful count of 0.
All matching duplicate rows are processed: no uniqueness constraint is implied.

## Agreed syntax

- Optional WHERE on SELECT, UPDATE, and DELETE.
- UPDATE table SET column = literal [, column = literal ...] [WHERE condition].
- DELETE FROM table [WHERE condition].
- Conditions: comparisons, IS NULL, IS NOT NULL, AND, OR, and parentheses.
- Comparison operands: column references or literals. Same-type column-to-column
  and literal-to-column comparisons are supported, not just column-on-the-left.
- INTEGER/TEXT comparisons: =, <>, !=, <, <=, >, >=. BOOLEAN comparisons: =, <>, !=.
- BOOLEAN columns and TRUE/FALSE/NULL literals may be predicates on their own.
  WHERE must resolve to BOOLEAN (a bare NULL predicate is unknown).
- Bind comparisons/NULL tests more tightly than AND, and AND more tightly than OR.
  Parentheses override grouping. Reject chained comparisons such as 1 < id < 10.
- IS NOT NULL is supported; general unary NOT is deferred for this bounded subset.
- SET values are literals only, including NULL. Column-copy assignments, arithmetic,
  functions, LIKE/IN/BETWEEN, RETURNING, aliases, ordering, and subqueries are deferred.

Literals reuse signed int64 checks and strict Stage 2 typing. Assigning text to
an integer column is an error. Duplicate SET target columns are rejected.
NULL literals take the required operand/target type; a comparison between two
NULL literals produces unknown. Unsupported/type-invalid expressions are rejected
before scanning, including on empty tables and unreachable logical branches.

## Pipeline and implementation responsibilities

```text
SQL -> tokens -> statement/expression AST -> bound typed expressions
    -> scan/filter/project or mutation plan -> executor -> memory row store
    -> structured result -> shell JSON
```

- Tokenizer/parser: add keywords/operators and expression nodes. Recursive descent
  uses separate OR, AND, comparison/NULL-test, and primary levels. Parentheses
  recursively parse expressions. Define and test a nesting limit (256)
  so pathological input is rejected with a syntax error instead of stack exhaustion.
- Analyzer: bind every column reference to a schema position, prepare typed
  literals, require boolean conditions, check operator compatibility, and validate
  all assignments before execution. Preserve existing error-type boundaries.
- Expression evaluator: consume bound expressions and one original row, returning
  typed values including nullable BOOLEAN. Keep it independent of shell/storage.
  Share the same condition evaluator across SELECT, UPDATE, and DELETE.
- Planner: extend existing small plans with optional predicates and add update/
  delete descriptions. SELECT logically performs Scan -> Filter -> Project.
  Do not filter after projection: WHERE may reference a column absent from SELECT.
- Executor: scan a snapshot, keep only rows whose condition is true, and build query
  results or replacement table rows. A missing WHERE means every row matches.
- Memory store: retain Append/Scan ownership contracts and add a minimal table-row
  replacement operation for mutations. No row IDs or per-row index addressing are
  needed before persistence. Revisit the boundary during Stage 5 rather than
  claiming whole-table replacement is the final disk mutation strategy.
- Engine/shell: route new commands and display updated/deleted acknowledgements.
  Mutation results must include affected_rows even when it is zero; the current
  integer field's omitempty behavior must change (for example, an optional count).
  SELECT/CREATE need not gain a meaningless affected-row field.

## SQL NULL behavior

A condition has three possible outcomes: true, false, or unknown. Unknown is
represented by a BOOLEAN NULL, not by inventing a fourth database data type.
Only true rows match WHERE; false and unknown do not.

Ordinary comparisons involving NULL yield unknown, including NULL = NULL.
Use IS NULL or IS NOT NULL to test absence explicitly. These tests always return
a non-null boolean. Compound logic follows SQL three-valued logic:

| Left | Right | AND | OR |
| --- | --- | --- | --- |
| true | true | true | true |
| true | false | false | true |
| true | unknown | unknown | true |
| false | false | false | false |
| false | unknown | false | unknown |
| unknown | unknown | unknown | unknown |

The remaining cases follow symmetry. A false predicate may short-circuit AND;
a true predicate may short-circuit OR. Unknown cannot be treated as false while
evaluating the compound expression. All branches are analyzed before execution,
regardless of runtime short-circuiting.

References for the adopted SQL behavior:
[PostgreSQL logical operators](https://www.postgresql.org/docs/current/functions-logical.html),
[comparison and NULL predicates](https://www.postgresql.org/docs/current/functions-comparison.html).
Our smaller supported operator set and type rules remain project decisions.

## Update/delete state and failure handling

Agreed initial memory implementation:

1. Parse and analyze the entire command before changing rows.
2. Scan an owned snapshot of the original table.
3. Evaluate the predicate against each original row exactly once.
4. UPDATE copies matching rows and applies prepared literal assignments; DELETE
   omits matching rows while retaining all other rows.
5. After successful evaluation, publish the complete replacement row collection.
6. Report the number of matched rows (UPDATE counts matches even if values were
   unchanged; DELETE counts rows removed).

If parsing, analysis, scanning, or evaluation fails before publication, the
current memory table stays unchanged. Memory replacement constructs owned copies
before its final map assignment. This is a local in-memory implementation property,
not general transaction support or a disk/crash guarantee. Storage failure
semantics must be revisited for Stage 5. Process termination/out-of-memory cannot
be treated as recoverable application errors.

Do not identify rows by their values: duplicate rows are separate occurrences.
Do not delete from a growing/shrinking slice while advancing an index; building
survivors from a snapshot avoids skipped adjacent matches. Evaluate WHERE before
changing a row so changing the predicate column cannot change its own match result.

## Edge cases: what can go wrong and agreed handling

| Case | Risk | Agreed handling / verification |
| --- | --- | --- |
| name = NULL or NULL = NULL | Treating NULL as an ordinary comparable value | Produce unknown; neither matches WHERE; test IS NULL separately |
| false AND unknown; true OR unknown | Treating unknown as ordinary false | Use complete three-valued truth tables and tests |
| A OR B AND C | Incorrect grouping changes affected rows | Parse as A OR (B AND C); test explicit alternative parentheses |
| SELECT name WHERE id = 2 | Filtering projected rows loses id | Evaluate predicate on complete stored row before projection |
| WHERE 42 or id = '42' | Implicit truthiness or type conversion | Reject during analysis, even on an empty table |
| TRUE OR missing = 1 | Short-circuiting hides invalid SQL | Bind and validate all branches before execution |
| Empty table or no matches | Missing result metadata or mistaken errors | SELECT returns columns plus []; mutations return affected_rows: 0 |
| UPDATE t SET a = 1, a = 2 | Ambiguous assignment order | Reject duplicate target columns before mutation |
| UPDATE changes its WHERE column | Evaluating mutated data changes matches | Evaluate once on original rows, then apply assignments |
| UPDATE sets the existing value | Unclear row count | Count matched rows, including unchanged values |
| DELETE matches consecutive/all/duplicate rows | Slice shifting skips rows | Build survivors; verify counts and every removed occurrence |
| No WHERE on UPDATE/DELETE | Unexpected subset behavior | Explicitly support all-row mutation; document and test it |
| Overflow or invalid assignment after valid targets | Partial updates from late validation | Analyze every target/value before execution |
| Comparison at min/max int64 | Subtraction-based comparison overflows | Compare using relational operators; test boundaries |
| Mixed case/Unicode text | Implicit locale/case assumptions | Case-sensitive Go string byte order; no collation/normalization promised |
| NULL, false, 0, empty string | Conflating absence and concrete values | Preserve typed values and test every distinct case |
| Deep expressions/malformed operators | Stack exhaustion or confusing parses | Explicit nesting limit, positioned syntax errors, lexer/parser tests |
| Store/evaluation failure before publication | Partial in-memory changes | Build replacements before publishing; inject failures and compare rows |
| Returned/mutation input slice changes | Hidden data corruption through sharing | Preserve owned copies, including replacement-store tests |

## Acceptance criteria and verification

Unit tests:

- Tokens/AST: each operator, longest-match tokens, UPDATE/DELETE, parentheses,
  precedence, malformed input, chained comparisons, and nesting-limit boundaries.
- Analysis: resolved positions, type compatibility, NULL inference, unknown names,
  invalid booleans, duplicate assignments, overflow, and validation of all branches.
- Evaluation: complete truth tables, NULL tests/comparisons, integer boundaries,
  case-sensitive text, comparisons in either operand order, and same-type columns.
- Plans/execution/store: filter-before-project, assignments, survivor construction,
  affected counts, replacement ownership, and failure before publication.

SQL integration tests:

- Existing Stage 3 examples remain valid.
- SELECT/UPDATE/DELETE share the same match behavior on the same original data.
- Parenthesized AND/OR predicates, NULL/empty/false/zero, missing WHERE, no matches,
  duplicate/adjacent matches, multiple assignments, and repeat mutations.
- Rejected commands followed by valid queries/mutations; verify unchanged rows.
- Table/session isolation, owned results, exact JSON zero counts, and shell recovery.

Format changed Go files and run go test ./..., go vet ./..., and go build ./....
Add examples/stage-4.sql and verify it using the built executable. Record actual
results only after implementation. Keep the 4–5-line header in every source/test
file. At the original design checkpoint no Stage 4 code or verification existed;
the results below apply to the completed implementation.

## Design decisions agreed with the user

- SET accepts literal values only, including NULL; arithmetic and column-copy
  assignments remain deferred.
- Follow standard SQL NULL semantics: comparisons involving NULL produce unknown,
  WHERE matches only true, and IS NULL / IS NOT NULL test absence explicitly.
- Prepare all memory-table changes before applying them. Analyze the whole command,
  evaluate against original rows, and build owned replacements before publication.
  Failures before publication leave existing memory rows unchanged; this does not
  introduce transactions or disk/crash guarantees.
- Retain the bounded syntax, type/operator rules, matched-row counts, and edge-case
  handling described above. Exact Go names/package layout are implementation details.

## Implementation map and ownership

- internal/sql/expression.go: expression AST and recursive-descent precedence
  levels; 256 limits on parentheses nesting and expression tree depth, including
  long unparenthesized AND/OR chains. Lexer normalizes != into <> and preserves
  the original lexeme. UPDATE/DELETE parsing keeps SET literal-only.
- internal/analyzer/expression.go: complete branch validation, resolved positions,
  strict operator types, contextual NULL, and BOOLEAN WHERE requirements.
  analyzer.go validates all assignments before returning a bound mutation.
- internal/expression: row evaluation, explicit three-valued logic, NULL tests,
  overflow-free integer ordering, and case-sensitive text comparisons. Node fields
  are private. Clone copies trees when planning so replacing a command node cannot
  change its plan; literals retain the existing immutable typed values.
- internal/planner: copied predicates and assignment slices, filtered Select,
  Update/Delete plans. CREATE/INSERT paths retain their established behavior.
- internal/executor/mutation.go: scan original snapshot, evaluate matches once,
  construct updated/survivor rows, and call Replace only after successful
  preparation. Zero matches return success without unnecessary replacement.
- internal/storage/memory.go: Replace builds owned row copies before its final
  map assignment. Append/Scan copying remains intact. Failure behavior of future
  disk backends is not inferred from the memory implementation.
- Result.AffectedRows is now *int: nil omits the field for CREATE/SELECT; mutation
  counts serialize explicitly even when zero. UPDATE counts matched rows.
- examples/stage-4.sql demonstrates the stage. scripts/verify_stage4.py checks the
  built executable against examples and a deterministic supported SQL workload
  evaluated independently by Python's SQLite. SQLite is verification-only.

## Verification results (2026-10-30)

Passed:

- gofmt; git diff --check.
- go test ./... -cover: SQL 99.7%, analyzer 96.3%, expression 94.4%, executor 94.5%,
  engine 86.7%; planner, storage, shell, catalog, and types 100%. cmd/db has no direct
  unit tests; the built entry point is exercised by real-process checks.
- go vet ./...; go build ./....
- go build -o /tmp/relational-db-stage4 ./cmd/db.
- python3 scripts/verify_stage4.py /tmp/relational-db-stage4: all 16 Stage 4 example
  commands and 11 Stage 3 regression example commands pass; 341 deterministic SQL
  commands agree with SQLite, including 208 row-result comparisons and 132
  affected-row counts. Rows are compared as multisets, preserving duplicates.
- go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2:
  268,556 generated inputs; no failure.
- Unit/integration cases cover full truth tables, operator grouping, contextual
  NULL, strict type errors on empty data, all-branch analysis, count zero, literal
  boundaries, original-row predicates, duplicate/adjacent deletion, all-row
  operations, session/table isolation, owned replacements/predicates, injected scan/
  evaluation/assignment/replace failures, and incomplete mutation rejection.

No disk persistence, indexes, transaction recovery, or concurrency is added.
Next is Stage 5 design discussion before choosing a disk format or implementing it.
