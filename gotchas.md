# Gotchas

## Pre-commit hooks run `go test ./...` with untracked files present

The hooks lint the whole module (golangci-lint `--fix`) and run `go test -short -race ./...`. prek stashes unstaged edits to tracked files but leaves untracked files in place, so a new test file meant for a later commit runs against an earlier commit's code and fails the hook. When splitting work into several commits, move later commits' new files to `.scratch/held/` (gitignored, and Go skips dot-directories) and restore each before its own commit.

## CI lints with golangci-lint v2.7.2, not your local version

`.github/workflows/ci.yml` pins v2.7.2; the pre-commit hook uses whatever is installed. On 2026-10-06, v2.7.2 flagged five `prealloc` sites in `internal/storage/markdown*.go` that 2.13.2 passed, so a green local commit can still fail CI. Check with `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2 run ./...`. Preallocating the public `List*` results would change Markdown's empty-list MCP JSON from `null` to `[]` while SQLite stays `null`.

## The Homebrew cask strips quarantine, and the old formula still wins

Releases publish `Casks/bbs-mcp.rb` (GoReleaser `homebrew_casks`, pushed with the `HOMEBREW_TAP_TOKEN` secret). The binary is not Developer ID signed or notarized, so the cask's postflight hook runs `xattr -dr com.apple.quarantine` on it; that bypasses Gatekeeper by decision (2026-10-06) and should go once releases are signed. The tap also still holds `Formula/bbs-mcp.rb`, and Homebrew installs a same-named third-party formula ahead of the cask (`Library/Homebrew/cli/named_args.rb`), so remove that formula from `harperreed/homebrew-tap` after the first cask release; existing formula users must `brew uninstall bbs-mcp` before `brew install --cask harperreed/tap/bbs-mcp`.
