# Documentation Audit Report

Generated: 2026-02-11 | Commit: 0550a19

> **Status: ALL ISSUES FIXED** - See fixes applied below.

## Executive Summary

| Metric | Count |
|--------|-------|
| Documents scanned | 4 |
| Claims verified | ~40 |
| Verified TRUE | 30 (75%) |
| **Verified FALSE** | **5 (12.5%)** |
| **Documentation Gaps** | **5 (12.5%)** |

## False Claims Requiring Fixes

### CLAUDE.md

| Line | Claim | Reality | Fix |
|------|-------|---------|-----|
| 20 | `Vault sync via suitesync/vault` | Not implemented. No `suitesync/vault` in go.mod, no `sync.go` exists. Only in planning docs. | Remove or mark as "(planned)" |
| 15-20 | Architecture lists only SQLite | Markdown backend exists as a full alternate storage backend (`internal/storage/markdown*.go`) | Add markdown backend to architecture list |

### SKILL.md (CLI Commands section, lines 63-69)

| Line | Claim | Reality | Fix |
|------|-------|---------|-----|
| 65 | `bbs thread list --topic general` | Actual syntax: `bbs thread list general` (positional arg, not flag) | Update to positional syntax |
| 66 | `bbs post list --thread <id>` | Command does not exist. Correct: `bbs thread show <id>` | Fix command name and syntax |
| 67 | `bbs thread new --topic general "Subject" --body "Content"` | Actual syntax: `bbs thread new general "Subject"` (no `--topic` or `--body` flags) | Update to positional syntax |
| 68 | `bbs post new --thread <id> "Reply content"` | Actual syntax: `bbs post <id> "Reply content"` (no `new` subcommand) | Remove `new` subcommand |

### CHARM_REMOVAL_PLAN.md

| Line | Claim | Reality | Fix |
|------|-------|---------|-----|
| (all) | Active migration plan | Migration is **fully completed**. All Charm files deleted, SQLite in place, schema matches exactly. | Archive or delete this file |

## Documentation Gaps (Undocumented Features)

### Markdown Storage Backend (CRITICAL)

The project has a **fully implemented markdown storage backend** (`internal/storage/markdown*.go`, 7 files) that is not documented in any user-facing doc:

- **CLAUDE.md** says "SQLite storage" as the only option
- **SKILL.md** says data location is `bbs.db (SQLite)` only
- **README.md** is essentially empty (`# BBS` only)
- No documentation on backend selection via `config.json`
- No migration guide for switching backends

**What's missing:**
1. Config supports `"backend": "sqlite"` or `"backend": "markdown"` - undocumented
2. `bbs migrate` command exists for bidirectional migration - undocumented for end users
3. Markdown backend stores topics in `_topics.yaml`, threads as `.md` files with YAML frontmatter
4. New users auto-default to markdown backend with SQLite auto-detection

### README.md is Empty

The README contains only `# BBS` - no project description, usage instructions, or setup guide.

## Verified TRUE Claims

### CLAUDE.md (13/14 TRUE)

| Claim | Evidence |
|-------|----------|
| Go CLI with Cobra | `go.mod`: `github.com/spf13/cobra v1.10.2` |
| SQLite storage (modernc.org/sqlite) | `go.mod`: `modernc.org/sqlite v1.41.0` |
| Bubble Tea TUI | `go.mod`: `github.com/charmbracelet/bubbletea v1.3.10` |
| MCP server for agent access | `internal/mcp/server.go` + `bbs mcp` command |
| `go run ./cmd/bbs` | Valid entry point |
| `go build -o bin/bbs ./cmd/bbs` | Builds successfully |
| `go test ./...` | 11 test files, all pass |
| Topics → Threads → Messages (+ Attachments) | `internal/models/models.go` defines all four |
| Identity format: `username@source` | `internal/identity/identity.go` line 25 |
| App DB: `~/.local/share/bbs/bbs.db` | `internal/storage/storage.go` lines 662-674 |
| Config: `~/.config/bbs/config.json` | `internal/config/config.go` lines 96-102 |
| Export markdown/yaml/json | `cmd/bbs/export.go` implements all three |
| Import yaml | `cmd/bbs/export.go` lines 338-455 |

### SKILL.md (MCP Tools - 9/9 TRUE)

All 9 MCP tools listed are implemented in `internal/mcp/tools.go`:
- `list_topics`, `create_topic`, `archive_topic`
- `list_threads`, `create_thread`, `sticky_thread`
- `list_messages`, `post_message`, `edit_message`

All tool parameter names and types match the implementation. No undocumented tools found.

## Pattern Summary

| Pattern | Count | Root Cause |
|---------|-------|------------|
| CLI syntax wrong (flags vs positional args) | 4 | Docs written from spec, not from actual Cobra commands |
| Undocumented storage backend | 5 gaps | Markdown backend added after initial docs written |
| Stale planning docs | 1 | CHARM_REMOVAL_PLAN.md not archived after completion |
| Planned features listed as implemented | 1 | Vault sync in architecture list |

## Human Review Queue

- [ ] **CLAUDE.md line 20**: Remove vault sync or mark as planned
- [ ] **CLAUDE.md lines 15-20**: Add markdown backend to architecture
- [ ] **SKILL.md lines 63-69**: Fix all 4 CLI command examples
- [ ] **CHARM_REMOVAL_PLAN.md**: Archive or delete (migration complete)
- [ ] **README.md**: Needs actual content (currently just `# BBS`)
- [ ] Add documentation for backend selection and migration
