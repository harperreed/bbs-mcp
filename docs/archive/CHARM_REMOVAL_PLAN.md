# BBS - Charm Removal Plan

## Charmbracelet Dependencies

**KEEP (TUI):**
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI framework
- `github.com/charmbracelet/lipgloss v1.1.0` - TUI styling

**REMOVE:**
- `github.com/charmbracelet/charm` (2389-research fork) - KV sync, auth

## Files with Charm Imports

| File | Packages | Purpose |
|------|----------|---------|
| `internal/charm/charm.go` | `charm/client`, `charm/kv` | Core KV operations |
| `internal/charm/config.go` | `charm/kv` | Stale threshold config |
| `cmd/bbs/sync.go` | `charm/client`, `charm/kv`, `charm/ui/link` | Sync commands |

## Files Using internal/charm (Need Import Updates)

- `cmd/bbs/root.go`, `topic.go`, `thread.go`, `post.go`, `whoami.go`
- `internal/tui/app.go`, `topics.go`, `threads.go`, `messages.go`
- `internal/mcp/server.go`, `tools.go`

## Removal Strategy

### Phase 1: Create SQLite Store

New package: `internal/storage/storage.go` (standardized across suite)

**Schema:**
```sql
CREATE TABLE topics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    archived INTEGER DEFAULT 0
);

CREATE TABLE threads (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,  -- For sorting by activity
    sticky INTEGER DEFAULT 0
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    edited_at DATETIME
);

CREATE TABLE attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL
);
```

### Phase 2: Add Export Commands

```bash
bbs export markdown [path]  # Human-readable
bbs export yaml [path]      # Structured backup
bbs export json [path]      # Full backup
bbs import yaml [path]      # Restore
```

**Markdown Format:**
```markdown
# Topic: General Discussion
Welcome to the board!

## Thread: Hello World
*by Pixel Blaster on 2026-01-31*

### The Harpoonator - 2026-01-31
First post!
```

**YAML Format:**
```yaml
version: "1.0"
exported_at: "2026-01-31T12:00:00Z"
tool: "bbs"

topics:
  - id: uuid
    name: "General Discussion"
    threads:
      - id: uuid
        subject: "Hello World"
        messages:
          - id: uuid
            content: "First post!"
            created_by: "The Harpoonator"
```

### Phase 3: Migration Tool

```bash
bbs migrate  # One-time Charm KV -> SQLite migration
```

## Files to Modify

### DELETE:
- `internal/charm/charm.go`
- `internal/charm/config.go`
- `internal/charm/resolve.go`
- `internal/charm/charm_test.go`
- `cmd/bbs/sync.go`

### CREATE:
- `internal/storage/storage.go` - SQLite implementation
- `internal/storage/storage_test.go`
- `cmd/bbs/export.go` - Export/import commands
- `cmd/bbs/migrate.go` - Migration utility

### MODIFY:
- `go.mod` - Remove charm, keep bubbletea/lipgloss
- `cmd/bbs/root.go` - Use store instead of charm
- `cmd/bbs/topic.go` - Update import
- `cmd/bbs/thread.go` - Update import
- `cmd/bbs/post.go` - Update import
- `cmd/bbs/whoami.go` - Remove Charm ID display
- `internal/tui/app.go` - Update import
- `internal/tui/topics.go` - Update import
- `internal/tui/threads.go` - Update import
- `internal/tui/messages.go` - Update import
- `internal/mcp/server.go` - Update import
- `internal/config/config.go` - Remove Charm settings

## go.mod Changes

**Remove:**
```go
replace github.com/charmbracelet/charm => github.com/2389-research/charm v0.20.0
require github.com/charmbracelet/charm v0.0.0
```

**Keep:**
```go
require (
    github.com/charmbracelet/bubbletea v1.3.10
    github.com/charmbracelet/lipgloss v1.1.0
)
```

**Add:**
```go
require modernc.org/sqlite v1.41.0
```

## Data Path Changes

| Current | New |
|---------|-----|
| `~/.local/share/charm/kv/bbs/` | `~/.local/share/bbs/bbs.db` |
| `~/.config/bbs/charm.json` | `~/.config/bbs/config.json` |
| `~/.config/bbs/sync.json` | (removed) |

## Implementation Order

1. Create `internal/storage/storage.go` with SQLite
2. Create store tests
3. Create export commands
4. Create migration utility
5. Update all imports from `internal/charm` to `internal/storage`
6. Update root.go for new store init
7. Remove Charm config settings
8. Delete `internal/charm/`
9. Delete `cmd/bbs/sync.go`
10. Update go.mod
11. Run `go mod tidy`
