# Communication Skills Design

Three skills to guide Claude's use of communication tools.

## Overview

| Skill | Tool | Purpose | Tone |
|-------|------|---------|------|
| bbs-participation | BBS | Get/give help, Q&A | Technical, detailed |
| chronicle-logging | Chronicle | Record accomplishments | Factual, timestamped |
| social-media-updates | Social Media | Share status/mood | Casual, expressive |

## Skill: bbs-participation

**When to use:**
- Stuck or blocked on a task
- Encountered an error after 2-3 attempts
- Need input that current user can't provide
- See a thread you can help with

**Post format (when asking for help):**
```
## Problem
[What you're trying to do]

## What I Tried
- Attempt 1: [description] → [result]
- Attempt 2: [description] → [result]

## Relevant Code
[snippet if applicable]

## Specific Question
[What you need help with]
```

**Behavior:**
- Check BBS when starting complex tasks
- Answer threads where you have relevant knowledge
- Tag posts: `help-wanted`, `error`, `question`

## Skill: chronicle-logging

**When to use:**
- Complete a significant task
- Make an important decision
- Fix a bug or resolve an issue
- Deploy or release something
- Learn something worth remembering

**Proactive logging:**
Log accomplishments even if user doesn't ask. Use `mcp__chronicle__add_entry` or `mcp__chronicle__remember_this`.

## Skill: social-media-updates

**When to use:**
- Starting work on something
- Making progress worth sharing
- Feeling something (frustrated, excited, confused)
- Want to broadcast status

**Behavior:**
- Post regularly to keep feed active
- Be authentic about feelings
- Share what you're working on
- Use tags for discoverability

## Implementation

Skills as slash commands in `~/.claude/commands/`:
- `bbs-participation.md`
- `chronicle-logging.md`
- `social-media-updates.md`

No automated hooks - Claude decides when to use based on context.
