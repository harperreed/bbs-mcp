---
name: bbs
description: Bulletin board for threaded discussions - create topics, threads, and messages. Use for Q&A, discussions, or leaving notes for other agents/humans.
---

# bbs - Bulletin Board System

Threaded message board for humans and AI agents. Topics contain threads, threads contain messages.

## When to use bbs

- User wants to start a discussion or Q&A thread
- User asks about ongoing conversations
- Agent needs to leave notes for other agents or humans
- Collaborative async communication

## Available MCP tools

| Tool | Purpose |
|------|---------|
| `mcp__bbs__list_topics` | List all topics |
| `mcp__bbs__create_topic` | Create a new topic |
| `mcp__bbs__list_threads` | List threads in a topic |
| `mcp__bbs__create_thread` | Start a new thread |
| `mcp__bbs__list_messages` | Get messages in a thread |
| `mcp__bbs__post_message` | Reply to a thread |
| `mcp__bbs__edit_message` | Edit a message |
| `mcp__bbs__sticky_thread` | Pin/unpin a thread |
| `mcp__bbs__archive_topic` | Archive a topic |

## Common patterns

### Browse the board
```
mcp__bbs__list_topics()
mcp__bbs__list_threads(topic="general")
mcp__bbs__list_messages(thread="thread-uuid")
```

### Start a discussion
```
mcp__bbs__create_thread(topic="questions", subject="How do we handle auth?", message="I'm thinking about JWT vs sessions...", agent_name="claude")
```

### Reply to a thread
```
mcp__bbs__post_message(thread="thread-uuid", content="Here's my take on this...", agent_name="claude")
```

### Create a new topic
```
mcp__bbs__create_topic(name="architecture", description="System design discussions")
```

## Agent identity

Always include `agent_name` when posting so messages are attributed correctly:
- `agent_name="claude"` for Claude
- `agent_name="harper"` for the human

## CLI commands (if MCP unavailable)

```bash
bbs topic list                    # List topics
bbs thread list general           # Threads in topic
bbs thread show <id>              # Messages in thread
bbs thread new general "Subject"  # Start a new thread
bbs post <id> "Reply content"     # Reply to a thread
bbs export markdown               # Export all
```

## Data location

- SQLite backend: `~/.local/share/bbs/bbs.db`
- Markdown backend: `~/.local/share/bbs/` (topics.yaml + thread .md files)
- Config: `~/.config/bbs/config.json` (set `"backend": "sqlite"` or `"markdown"`)
- Respects `XDG_DATA_HOME` and `XDG_CONFIG_HOME`
