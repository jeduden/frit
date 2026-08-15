# CLAUDE.md

## Project

frit — a command and control CLI over plans, worktrees, hosts and
agents, written in Go.

## Docs

- [Plan template; see PLAN.md for status, plans live in plan/](plan/proto.md)

## Architecture

frit consumes rather than reimplements. Three tools already own a
layer of this problem, and frit is the join between them:

- **mdsmith** owns markdown, and is imported as a library, not run
  as a subprocess. `pkg/markdown` splits front matter from body and
  hands back the AST, so frit and mdsmith always agree on where a
  document's front matter ends — including the awkward cases, like a
  block scalar containing a line of three dashes. A subprocess per
  file would be thousands of forks for one walk.
  What the public API does *not* expose is `extract`'s CUE schema
  projection and the `deps` link graph; both live in `internal/`.
  If frit needs link-following later, that is the moment to ask for
  them to be promoted, not to hand-roll a second parser.
- **herdr** owns panes, worktrees and prompts. frit reads its socket
  API for live agent state and hands panes back to it for anything
  interactive.
- **plan-lane** owns claims. A hold is a ref name, so frit reads
  holds with `git ls-remote` and delegates every mutation.

The rule that keeps those boundaries honest: frit indexes and
displays. It never edits a plan, never spawns an agent it does not
hand straight back, and never reads a transcript.

## Development Workflow

- Any change follows Red/Green TDD: failing test, then pass, then commit
- Keep commits small and focused on one change
- Run `mdsmith check .` before committing; all markdown must pass
- Never modify `.mdsmith.yml` (linter configuration) without explicit
  user consent

## Plan Maintenance

When implementing work tracked by `plan/`:

- Update the plan file **as part of implementation**, not a follow-up
- Check off tasks and acceptance criteria as they're verified
- Move front-matter `status`: `🔲` → `🔳` on start, `✅` when done
- If implementation deviates, update plan text to match
- Run `mdsmith fix PLAN.md` after editing front matter

## Configuration

Every setting resolves from four places, most specific first:

1. the command line (`--root`)
2. the environment (`FRIT_ROOT`)
3. `.frit.yml` beside the work, or `$FRIT_CONFIG`
4. the user config, `$XDG_CONFIG_HOME/frit/config.yml`

kong's own `env:` tag sits *beneath* its config resolver, which
would let a stale file outrank the environment. `envResolver` in
[cmd/frit/main.go](cmd/frit/main.go) restores the expected order and
is pinned by a precedence test — do not remove it.

## Build & Test Commands

Requires Go 1.25+. Dev tools build from `tools/go.mod` so their
dependency trees never constrain consumers of this module.

- `go build ./...` — build all packages
- `go test ./...` — run all tests
- `go test -run TestName ./...` — run a specific test
- `go vet ./...` — run go vet
- `go tool -modfile=tools/go.mod golangci-lint run` — lint
- `go mod tidy -modfile=tools/go.mod` — tidy the tools module
- `go run ./cmd/frit repos` — list discovered repos and worktrees
- `go run ./cmd/frit plans` — read plan files off every ref

## Project Layout

Follows the standard Go project layout:

- `cmd/frit/` — main entry point
- `internal/` — private packages
- `plan/` — plan files, indexed by PLAN.md

## Code Style

- Follow standard Go conventions (gofmt, goimports)
- Keep functions small and focused
- Error messages: lowercase, no trailing punctuation
- Prefer returning errors over panicking
- Every function ships with a dedicated unit test

## Defensive Code

Add a defensive branch only when you can drive it red/green. Write
the failing test first. Then add the code that takes the branch.

## Shelling Out To Git

Git is invoked as a subprocess, never through a library. Two rules
keep that safe:

- Always pass `-C <dir>`; never rely on the process working
  directory, because frit walks many repositories in one run.
- Parse porcelain formats only (`--porcelain`, `-z`, explicit
  `--format`). Human-readable git output is not a stable contract.
