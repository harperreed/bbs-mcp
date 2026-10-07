# Gotchas

## Pre-commit hooks run `go test ./...` with untracked files present

The hooks lint the whole module (golangci-lint `--fix`) and run `go test -short -race ./...`. prek stashes unstaged edits to tracked files but leaves untracked files in place, so a new test file meant for a later commit runs against an earlier commit's code and fails the hook. When splitting work into several commits, move later commits' new files to `.scratch/held/` (gitignored, and Go skips dot-directories) and restore each before its own commit.

## CI pins golangci-lint; your local copy floats

`.github/workflows/ci.yml` pins golangci-lint v2.13.2, but the pre-commit hook runs whatever binary is installed, and Homebrew upgrades it on its own. Versions disagree: the old v2.7.2 pin flagged five `prealloc` sites in `internal/storage/markdown*.go` that 2.13.2 passes, which kept CI red while local commits were green. When `golangci-lint version` stops matching the pin, either bump the pin or run the pinned version with `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<pin> run ./...`. Don't "fix" those prealloc sites by preallocating the public `List*` results: that changes Markdown's empty-list MCP JSON from `null` to `[]` while SQLite stays `null`.

## The Homebrew cask strips quarantine, and the old formula still wins

Releases publish `Casks/bbs-mcp.rb` (GoReleaser `homebrew_casks`, pushed with the `HOMEBREW_TAP_TOKEN` secret). The binary is not Developer ID signed or notarized, so the cask's postflight hook runs `xattr -dr com.apple.quarantine` on it; that bypasses Gatekeeper by decision (2026-10-06) and should go once releases are signed. The tap also still holds `Formula/bbs-mcp.rb`, and Homebrew installs a same-named third-party formula ahead of the cask (`Library/Homebrew/cli/named_args.rb`), so remove that formula from `harperreed/homebrew-tap` after the first cask release; existing formula users must `brew uninstall bbs-mcp` before `brew install --cask harperreed/tap/bbs-mcp`.
