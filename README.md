# Relational DB

A relational database engine built from scratch in Go to understand storage,
indexes, and SQL execution through a carefully tested implementation.

The first phase is an embedded engine with a SQL shell, persistent tables,
single-table queries, and indexes. The project is currently in planning; no
database functionality has been implemented.

Start with the [SQL-to-storage walkthrough](docs/sql-to-storage-overview.md), then
read the [Phase 1 high-level plan](docs/phase-1-plan.md). We build from SQL downward,
using in-memory execution before adding persistence. Detailed stage designs will
be discussed and recorded before their implementation.

[AGENTS.md](AGENTS.md) records the project instructions for Codex, including clean
Go code, SOLID principles, and unit and integration tests in every implementation
stage.
