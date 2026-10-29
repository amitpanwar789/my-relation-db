# Stage 1: SQL Input and Parsing

Design agreed: 2026-10-29. Implementation status and verification are recorded below.

## Purpose and learning model

Turn SQL text into a structured description, without executing database operations.
For example, `SELECT id, name FROM users;` becomes a SelectStatement containing
the table `users` and columns `id` and `name`. Unknown tables still parse: deciding
whether a table exists belongs to Stage 2.

The path is: shell input -> tokenizer -> parser -> AST -> displayed JSON.

## Responsibilities and implementation choices

- `cmd/db/main.go`: connect standard input/output to the shell, choose interactive
  prompts when input is a terminal, and report terminal I/O failures.
- `internal/shell`: collect semicolon-terminated statements, respecting quoted
  strings, support multiline input and multiple statements, display results/errors,
  and continue after a rejected statement. Accept streams to enable integration
  tests without a real terminal. EOF exits; no extra shell command language yet.
- `internal/sql/token.go`: token kinds, original lexemes, normalized values, and
  positions (zero-based byte offset; one-based line and rune column).
- `internal/sql/lexer.go`: handwritten tokenizer. Recognize keywords, names,
  punctuation, signed integer literals, text, booleans, and NULL; report invalid
  characters/unterminated strings with source locations.
- `internal/sql/ast.go`: statement-specific structs and literal representations,
  independent of shell and storage. A small statement interface distinguishes AST
  node types; no interface is needed for every component.
- `internal/sql/parser.go`: handwritten recursive-descent parser. Functions follow
  grammar rules, using the next token to choose a statement or list element.

A handwritten parser keeps control flow and errors visible for learning. A parser
generator adds machinery for a grammar this small; an existing SQL parser would
hide much of the work we want to understand. Use only the standard library.

## Initial grammar

```text
statement = create_table | insert | select
create_table = CREATE TABLE identifier '(' column_def {',' column_def} ')'
column_def = identifier (INTEGER | TEXT | BOOLEAN)
insert = INSERT INTO identifier '(' identifier {',' identifier} ')'
         VALUES '(' literal {',' literal} ')'
select = SELECT ('*' | identifier {',' identifier}) FROM identifier
literal = signed_integer | string | TRUE | FALSE | NULL
```

The parser accepts exactly one statement with an optional final semicolon followed
by EOF. The shell requires a semicolon for submission. INSERT requires a column
list and exactly one VALUES row. Lists must be nonempty. Reject trailing tokens.
WHERE/UPDATE/DELETE are added in Stage 4; CREATE INDEX is added in Stage 6.

Examples:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (id, name, active) VALUES (42, 'Amit', TRUE);
SELECT * FROM users;
SELECT id, name FROM users;
```

## Lexical and AST rules

- Keywords are case-insensitive. Unquoted names use ASCII letters/underscore first,
  then letters/digits/underscore, and normalize to lowercase. Recognized keywords
  are reserved. Quoted identifiers, comments, aliases, and type aliases are deferred.
- Whitespace is space, tab, CR, or LF. CRLF counts as one line break via LF.
- Strings use single quotes, permit valid UTF-8 and embedded newlines/semicolons,
  and escape a quote by doubling it: `'O''Brien'`. Backslashes are ordinary text.
  Invalid UTF-8 is rejected. Delimiters inside strings have no structural meaning.
- Integers are optional adjacent + or - followed by decimal digits. Preserve the
  spelling as text in a typed integer-literal AST node; range checks belong to
  Stage 2. Fractions and arithmetic expressions are unsupported.
- Literal AST nodes distinguish integer, text, boolean, and NULL. The AST retains
  syntactic meaning, not original whitespace; source locations live on tokens/errors.
- Duplicate columns and INSERT arity/type mismatches remain semantic checks for
  Stage 2; Stage 1 must not reject them on semantic grounds.
- Print ASTs as indented JSON with a statement kind and explicit fields. Output
  describes parsing and never claims that a table/row was created or saved.

## Shell boundaries and errors

Collect until a semicolon outside a string. Doubled quotes remain inside strings.
Interactive mode shows `db> ` and a continuation prompt for multiline statements;
piped input omits prompts. Blank/empty statements are ignored. Position numbering
is relative to each submitted statement, including any leading whitespace.

Lexical and syntax errors identify the line/column and expected/found input where
applicable. Report errors without panicking, then process the next framed statement.
At EOF, incomplete nonblank input produces an error and is not silently executed.
An unmatched quote consumes the remainder of input; recovering inside such a
string is deliberately unsupported. Input/output failures propagate to the caller.
No custom input-size limit is introduced yet; this local learning shell is not a
network-facing service.

## Invariants and limits

- Parsing has no database, catalog, filesystem, or execution side effects.
- A successful parse consumes the complete input and returns one AST.
- Tokens always advance or reach EOF; malformed input must terminate with an error.
- String contents are preserved after SQL quote unescaping; names normalize only
  outside strings. Large integer literals do not lose precision through JSON numbers.
- The shell and parser are independently testable. No rows or schemas persist.

## Acceptance criteria and verification

- Unit-test token values/positions, escaping, signs, multiline and Unicode strings,
  invalid characters/UTF-8, and unterminated strings.
- Unit-test each AST shape, optional semicolons, keyword casing, wildcard/projection,
  all literal kinds, malformed grammar, trailing garbage, and error positions.
- Confirm unknown tables and semantic mismatches parse without catalog access.
- Integration-test streams through framing, tokenization, parsing, and JSON output:
  multiline statements, several statements, quoted semicolons/doubled quotes,
  recovery after errors, EOF handling, prompts, and failing input/output streams.
- Seed a parser fuzz test with valid and malformed SQL; assert no panic and that
  success always returns a statement.
- Format Go files; run `go test ./...`, `go vet ./...`, and `go build ./...`.
  Exercise the actual command with piped example SQL. Record results below.

## Implementation outcome

Implemented on 2026-10-29. The shell displays JSON ASTs for the agreed CREATE TABLE,
INSERT, and SELECT forms; semantic checks and execution remain deferred. The
implementation uses only Go's standard library. See `examples/stage-1.sql` for
a repeatable session.

Verification passed:

- `go test ./...`: tokenizer/parser unit tests and shell integration tests.
- `go vet ./...`: no findings.
- `go build -o /tmp/relational-db-stage1 ./cmd/db`: command compiled successfully.
- `go test ./internal/sql -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2`:
  passed, 440,569 generated executions in the recorded run, with no failure.
- Real-command smoke checks: all five statements in `examples/stage-1.sql`
  produced the expected AST kinds; quoted Unicode text round-tripped unchanged.
  A separate stream confirmed recovery after invalid SQL and a diagnostic for
  a final statement missing its semicolon.

The environment required build-cache access for Go checks; the initial restricted
test invocation could not write its cache. The subsequent checks above passed
with the required access.
