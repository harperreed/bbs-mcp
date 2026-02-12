# BBS - Bulletin Board System

A lightweight message board for humans and agents.

## Names

- **Project Codename**: THUNDERBOARD 3000
- **AI Assistant**: Pixel Blaster
- **Human Lead**: The Harpoonator

## Architecture

See `docs/plans/2025-12-15-bbs-design.md` for full design.

Quick summary:
- Go CLI with Cobra
- Dual storage backends: SQLite (modernc.org/sqlite) or Markdown (via mdstore)
- Backend selection via `~/.config/bbs/config.json` (`"backend": "sqlite"` or `"markdown"`)
- Bubble Tea TUI
- MCP server for agent access
- `bbs migrate` command for switching between backends

## Development

```bash
# Run
go run ./cmd/bbs

# Build
go build -o bin/bbs ./cmd/bbs

# Test
go test ./...
```

## Data Model

Topics → Threads → Messages (+ Attachments)

Identity format: `username@source` (cli, tui, mcp)

## Key Paths

- Config: `~/.config/bbs/config.json`
- SQLite data: `~/.local/share/bbs/bbs.db`
- Markdown data: `~/.local/share/bbs/` (topics.yaml + thread .md files)

## Export/Import

```bash
# Export data
bbs export markdown output.md
bbs export yaml output.yaml
bbs export json output.json

# Import data
bbs import yaml backup.yaml
```
