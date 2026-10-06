# Gotchas

## Pre-commit hooks run `go test ./...` with untracked files present

The hooks lint the whole module (golangci-lint `--fix`) and run `go test -short -race ./...`. prek stashes unstaged edits to tracked files but leaves untracked files in place, so a new test file meant for a later commit runs against an earlier commit's code and fails the hook. When splitting work into several commits, move later commits' new files to `.scratch/held/` (gitignored, and Go skips dot-directories) and restore each before its own commit.

## CI lints with golangci-lint v2.7.2, not your local version

`.github/workflows/ci.yml` pins v2.7.2; the pre-commit hook uses whatever is installed. On 2026-10-06, v2.7.2 flagged five `prealloc` sites in `internal/storage/markdown*.go` that 2.13.2 passed, so a green local commit can still fail CI. Check with `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2 run ./...`. Preallocating the public `List*` results would change Markdown's empty-list MCP JSON from `null` to `[]` while SQLite stays `null`.

## The Homebrew cask switch is on hold (2026-10-06)

Moving `.goreleaser.yml` from `brews` to `homebrew_casks` needs a `HOMEBREW_CASK_TOKEN` repo secret (only `HOMEBREW_TAP_TOKEN` exists), a decision on signing (an unsigned binary installed through a cask is blocked by Gatekeeper), and a plan for the existing `Formula/bbs-mcp.rb` in the tap, which shares the cask's name. `brews` is deprecated but stays until a GoReleaser major version.
