# Gotchas

## Pre-commit hooks run `go test ./...` with untracked files present

The hooks lint the whole module (golangci-lint `--fix`) and run `go test -short -race ./...`. prek stashes unstaged edits to tracked files but leaves untracked files in place, so a new test file meant for a later commit runs against an earlier commit's code and fails the hook. When splitting work into several commits, move later commits' new files to `.scratch/held/` (gitignored, and Go skips dot-directories) and restore each before its own commit.

## CI pins golangci-lint; your local copy floats

`.github/workflows/ci.yml` pins golangci-lint v2.13.2, but the pre-commit hook runs whatever binary is installed, and Homebrew upgrades it on its own. Versions disagree: the old v2.7.2 pin flagged five `prealloc` sites in `internal/storage/markdown*.go` that 2.13.2 passes, which kept CI red while local commits were green. When `golangci-lint version` stops matching the pin, either bump the pin or run the pinned version with `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<pin> run ./...`. Don't "fix" those prealloc sites by preallocating the public `List*` results: that changes Markdown's empty-list MCP JSON from `null` to `[]` while SQLite stays `null`.

## The Homebrew formula is deliberate; don't migrate to a cask on a whim

`.goreleaser.yml` publishes `Formula/bbs-mcp.rb` through GoReleaser's deprecated `brews` block, so `goreleaser check` exits non-zero, but releases still work (GoReleaser 2.18.2, 2026-10-06). v1.3.5 shipped a cask instead, and it went badly: the unsigned binary needed a postflight hook that strips quarantine (bypassing Gatekeeper, in a `postflight` form Homebrew 7.0.8 deprecates for `postflight_steps`), and Homebrew installs a same-named third-party formula ahead of a cask (`Library/Homebrew/cli/named_args.rb`), so formula users never moved over. We went back to the formula the same day. Revisit only when GoReleaser drops `brews` or releases are Developer ID signed and notarized; GoReleaser's documented formula-to-cask path adds `tap_migrations.json` and deletes the old formula.
