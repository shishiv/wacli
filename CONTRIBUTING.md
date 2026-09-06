# Contributing to wacli

Thank you for your interest in contributing to `wacli`! This document outlines our development process, architectural conventions, and contribution guidelines.

## Architectural Principles

1. **Two Databases**:
   - `session.db`: Managed entirely by `whatsmeow` for connection keys and cryptographic sessions.
   - `wacli.db`: Application database for contacts, chats, messages, groups, media metadata, and FTS5 search.
2. **Locking & Concurrency**:
   - Every write operation requires acquiring the `LOCK` file in the store directory (`~/.local/state/wacli` on Linux, `~/.wacli` fallback).
   - Long-running daemons (`wacli sync --follow`) hold this lock and open a local UNIX domain socket (`.send.sock`) to delegate actions from ephemeral CLI commands.
3. **Agent-Safe Read-Only Mode**:
   - When `--read-only` or `WACLI_READONLY=1` is set, write commands must exit immediately with an informative error. Read operations must never take the write lock or alter local or remote state.
4. **Minimal Dependencies**:
   - No external CGO dependencies beyond `go-sqlite3`.
   - Keep the dependency graph small and auditable.

---

## Local Development & Prerequisites

- **Go**: `1.27.0` (pinned security floor).
- **Node.js**: `>= 24`
- **pnpm**: `11.x`
- **C Compiler**: GCC or Clang (required for SQLite CGO).

### Common Commands

```bash
# Compile binary with FTS5 search support
pnpm build

# Run unit tests across all packages
pnpm test

# Format code (gofmt)
pnpm format
pnpm format:check

# Run linter
pnpm lint
```

---

## Standalone CI Gate (No GitHub Actions Required)

Before submitting any Pull Request, run the local hermetic CI runner:

```bash
./scripts/ci-local.sh
```

This executes all 8 gate checks in ~14 seconds:
1. Git diff & whitespace hygiene (`git diff --check`)
2. Go code formatting (`gofmt`)
3. Static analysis (`go vet`)
4. Standard library vulnerability audit (`govulncheck`)
5. Deadcode elimination check (`deadcode`)
6. Test matrix (plain Go, SQLite FTS5, Windows lock cross-compile, CGO requirement, doc tests)
7. E2E store & SQLC verification
8. Production build generation (`dist/wacli`)

---

## Commit & Pull Request Guidelines

- **Conventional Commits**: Use `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:` or `chore:` with an imperative summary (e.g., `fix(ipc): delegate mark-read through follow socket`).
- **Atomic Scope**: Keep PRs tightly focused on a single issue. Avoid combining unrelated fixes or feature additions.
- **Regression Tests**: Every bug fix or new command must include regression unit tests next to the code it covers (`*_test.go`). Use `fake_wa_test.go` and in-memory SQLite instances; avoid requiring live WhatsApp connections in tests.
- **PR Description**: Clearly state:
  - What changed
  - Why this change was made
  - Reproduction or proof steps (synthetic test output or transcript)
  - Any new CLI flags or configuration options
- **Attribution**: Include `Co-authored-by:` trailers when building on previous contributors' work.
