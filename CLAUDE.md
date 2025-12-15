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
- SQLite storage (modernc.org/sqlite)
- Bubble Tea TUI
- MCP server for agent access
- Vault sync via suitesync/vault

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

- App DB: `~/.local/share/bbs/bbs.db`
- Vault DB: `~/.config/bbs/vault.db`
- Sync Config: `~/.config/bbs/sync.json`
