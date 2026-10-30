# Project Progress

Updated: 2026-10-30

## Current state

- Phase 1 uses the agreed top-down order in phase-1-plan.md.
- Stage 1: implemented, tested, committed, and pushed as ab1b12f.
- Stage 2: implemented, verified, committed as b9bfd34, and pushed. Design/results: stage-2-catalog-semantic-design.md.
- Stage 3: design accepted, implemented, verified, committed as 856f4bc, and pushed. Design/results: stage-3-in-memory-execution-design.md.
- Stage 4: implemented and verified; included in the Stage 4 checkpoint commit. Design/results:
  stage-4-expressions-mutations-design.md.
- Stages 5–8: high-level plan only.

## What the application does today

The shell supports CREATE/INSERT, SELECT with WHERE, literal-only UPDATE, and
DELETE in memory. Conditions use strict types and SQL three-valued NULL logic,
comparison/AND/OR precedence, parentheses, and IS NULL / IS NOT NULL. Mutations
prepare changes before applying them and report explicit counts including zero.
Tables/sessions and owned results remain isolated. Exiting loses schemas/rows;
there are no database files. Go unit/integration tests, static checks, builds,
parser fuzzing, and a repeatable 341-command comparison against SQLite passed.

## Next action

Review Stage 4 behavior and commit/push when requested. Next is Stage 5 design:
persistent catalog/records, pages/files, row identifiers, mutation storage,
flush/close behavior, malformed files, and clean-reopen guarantees. Explain the
options and edge cases before choosing a disk format or implementing storage.
Stages 2 and 3 are already pushed.

## Repeatable workflow

For each stage: explain (including edge cases and handling) -> discuss options ->
save agreed design -> implement -> verify -> record outcome. Commit/push when requested or otherwise authorized.
Update this file when the stage's design or implementation status changes.
