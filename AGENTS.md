# Project instructions

## Read before implementation

- Read `docs/phase-1-plan.md` for the current scope and stage order.
- Read `docs/sql-to-storage-overview.md` for the conceptual architecture and
  top-down learning approach.
- Read the current stage's design document, when present. Detailed designs belong
  in `docs/stage-N-<topic>-design.md` and must match the current plan.
- Follow the user's latest instructions when they revise an earlier decision;
  update affected documentation so competing plans do not remain active.

## Learning and stage discipline

- Build from SQL downward: parsing, semantics, in-memory execution, persistence,
  indexes, and index-aware planning. Do not revert to storage-first implementation.
- Explain each stage with examples, discuss options, and record the agreed design
  before implementing that stage. Do not silently choose major unresolved designs
  or skip ahead into later stages. Once a stage is agreed, complete its authorized
  work without repeatedly asking for routine implementation choices.
- Each stage design must state scope, inputs/outputs, component responsibilities,
  invariants, errors/limitations, acceptance criteria, and verification commands.
- Whenever a design discussion is completed, save its explanation, examples,
  decisions, and tradeoffs in a Markdown design document under `docs/` before
  implementation. Keep an index in the README so the user can revisit discussions.
- Keep changes small and explain how the implementation maps to SQL behavior.
- Do not add transactions, concurrency, networking, permissions, or other deferred
  features without a scope decision. Do not claim crash safety in Phase 1.

## Clean code and design principles

- Write idiomatic, readable Go with clear names, explicit errors, and small,
  cohesive functions and packages. Prefer simple designs over clever machinery.
- Start every source and test file with a short comment, normally 4–5 lines,
  explaining its purpose, how it works, and its role in plain language. Keep the
  style consistent and update the explanation when the responsibility changes.
- Apply SOLID principles in a Go-appropriate way:
  - Single responsibility: give components one clear responsibility and reason
    to change; keep shell I/O, parsing, validation, execution, and storage separate.
  - Open/closed: use clear extension points when a real alternative is needed,
    rather than scattering special cases across unrelated components.
  - Liskov substitution: implementations of an interface must honor the same
    documented behavior, errors, and invariants. Test substitutable backends
    against shared contracts where useful.
  - Interface segregation: define small, focused interfaces around consumer needs.
  - Dependency inversion: keep query behavior independent of concrete file I/O
    and shell presentation through narrow boundaries where needed.
- Prefer composition; do not imitate class hierarchies or introduce interfaces
  for every type. Avoid speculative abstractions and premature optimization.
- Keep responsibilities separate without fragmenting a straightforward operation
  into unnecessary layers. Keep package dependencies acyclic.
- Centralize shared rules, avoid needless duplication, and document non-obvious
  invariants and tradeoffs. Comments should explain why, not restate the code.
- Validate untrusted SQL/file inputs at appropriate boundaries. Propagate errors
  with useful context; do not hide failures or use panics for expected errors.
- Implement core database behavior ourselves. Prefer the standard library and
  justify dependencies that materially affect the design.

## Tests are part of every implementation stage

- Include meaningful unit tests for stage-specific logic and integration tests
  exercising the stage's connected components. Add end-to-end SQL tests once
  execution exists. Test in the same change as implementation, not at release time.
- Cover normal behavior, boundaries, invalid inputs, errors, and relevant invariants.
  Assert observable contracts rather than private implementation details.
- Use Go's testing package and table-driven tests where they improve clarity.
  Keep tests deterministic and isolated; use temporary directories for disk tests.
- Test parsing independently from table existence; test semantic failures separately
  from syntax failures. Early tests must work without future storage components.
- Once persistence exists, verify clean close/reopen. Once indexes exist, compare
  indexed results with scans before and after mutations. Add regression tests for
  corrected defects. Use fuzz/property tests where they add meaningful coverage.
- Do not declare a stage complete until its acceptance criteria and relevant unit
  and integration tests pass. Report unrun checks and blockers honestly.
- Once a Go module exists, format changed Go files with gofmt and run
  `go test ./...` and `go vet ./...` before declaring implementation complete.
  Record additional stage-specific commands and results in the stage document.
  No Go checks are expected for documentation-only work before code exists.

## Documentation and completion

- Keep the README, high-level plan, and stage designs consistent with the code.
- Record decisions, supported SQL, limitations, and test evidence as they emerge.
- Report what changed, how it was verified, and any remaining limitations. Never
  imply a planned feature is already implemented.
