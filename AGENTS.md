# Agent Notes

<!-- Included content comes from CLAUDE.md. Edit that
     file first, then run `mdsmith fix .` to propagate. -->

Instructions for AI coding agents (Codex, Copilot,
Claude).

<?include
file: CLAUDE.md
strip-frontmatter: "true"
?>
# CLAUDE.md

## Project

frit — a command and control CLI over plans, worktrees, hosts and
agents, written in Go.

## Docs

- [Plan template; see PLAN.md for status, plans live in plan/](plan/proto.md)
- [Why frit exists — the research](docs/research/fleet-index/README.md)
- [Where the name comes from](docs/research/naming.md)

Research notes record how a decision was reached, including the wrong
turns, and are dated rather than kept current. When one has been
overturned it says so inline. This file and PLAN.md are the current
record; the research is the reasoning.

## Architecture

frit consumes rather than reimplements. Two tools already own a layer
of this problem, and frit is the join between them. It owns one
mutation itself, the claim:

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
- **The claim is frit's own.** A hold is a ref name, and frit mints
  it: an empty marker commit pushed with `--force-with-lease`, so a
  hold is atomic across machines and a lost race is caught rather than
  papered over ([internal/claim](internal/claim)). frit reads holds
  out of the ref list too — which names count is declared per
  repository in its `.frit.yml`, never inferred from a plan id, and
  refs merged into the default branch are excluded so landed work does
  not read as an active claim. frit still delegates the worktree and
  the pane the claim stands up to herdr; the lease is the one thing it
  writes.

The rule that keeps those boundaries honest: frit indexes, displays,
and owns exactly one mutation — the claim, because a hold has to be
atomic and a ref push is the only atom git offers. It never edits a
plan, never spawns an agent it does not hand straight back, and never
reads a transcript.

## Development Workflow

- Any change follows Red/Green TDD: failing test, then pass, then commit
- Keep commits small and focused on one change
- Run `mdsmith check .` before committing; all markdown must pass
- Never modify `.mdsmith.yml` (linter configuration) without explicit
  user consent
- Run `mdsmith merge-driver install` once per clone. `PLAN.md` is a
  mdsmith catalog, regenerated from the plan files rather than edited,
  so two branches adding a plan conflict on it by construction. The
  driver regenerates the catalog during a merge or rebase instead of
  leaving conflict markers. frit itself never reads `PLAN.md` — it
  enumerates the plan files directly — so this is repo hygiene, not a
  frit dependency. `.gitattributes` is committed; the driver is
  per-clone local config, which is why each clone installs it.

## Plan Maintenance

When implementing work tracked by `plan/`:

- Update the plan file **as part of implementation**, not a follow-up
- Check off tasks and acceptance criteria as they're verified
- Move front-matter `status`: `🔲` → `🔳` on start, `✅` when done
- If implementation deviates, update plan text to match
- Run `mdsmith fix PLAN.md` after editing front matter

## Configuration

There are two kinds of setting, and they are deliberately different.

**Per-repository settings** live in a `.frit.yml` committed inside
each repository frit indexes — `plan-dir` and the `holds` patterns.
They describe that project's conventions, so they travel with it
rather than living on one machine. `frit init` writes the file with
every default present and commented. A repository with no file gets
the defaults, so the canonical convention needs no config at all.

Hold patterns are a **list**, because conventions decorate the plan id
freely and one pattern would see only a fraction of a repository's
lanes. A ref matching no pattern is simply not a hold; that is the
honest answer, not a gap.

**frit's own settings** — `--root` and friends — resolve from four
places, most specific first:

1. the command line (`--root`)
2. the environment (`FRIT_ROOT`)
3. `.frit.yml` beside the work, or `$FRIT_CONFIG`
4. the user config, `$XDG_CONFIG_HOME/frit/config.yml`

kong's own `env:` tag sits *beneath* its config resolver, which
would let a stale file outrank the environment. `envResolver` in
[cmd/frit/main.go](cmd/frit/main.go) restores the expected order and
is pinned by a precedence test — do not remove it.

## The JSON Contract

`--json` is global, so every command answers it. Both renderings
are built from one model in [internal/report](internal/report),
never from two printers kept in step by hand: a command gathers
what it found into a document and then prints it as a table or
encodes it as JSON.

Three rules make that JSON something a consumer can be written
against, and `internal/report/testdata` pins them:

- every key is always present, so a field is indexed without
  first testing for it
- a list is `[]` and never null
- a repository frit could not read is carried in the document.
  Under `--json` nothing goes to stderr, because stdout is then
  the whole report

Two divergences from the table are deliberate. `--detail` decides
how much of the plan index a person is shown, while the document
always carries all of it; and the table drops a repository with
nothing to report, while the document keeps it with empty sets.

Re-record the golden files with `go test ./internal/report
-update`, and read the diff before committing it. Every consumer
of frit is written against those files.

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
- `go run ./cmd/frit orphans` — claims and checkouts that disagree
- `go run ./cmd/frit orphans --json` — the same report for an agent
- `go run ./cmd/frit who` — which lane has a live agent on it, from herdr
- `go run ./cmd/frit ready` — plans startable now: deps done, nobody holds
- `go run ./cmd/frit pick -n 5` — the same, ranked by how much each unblocks
- `go run ./cmd/frit next <id>` — the first phase of a plan not yet done
- `go run ./cmd/frit show <id>` — what blocks a plan; `--all` for every dep
- `go run ./cmd/frit find <text>` — search plan titles and summaries
- `go run ./cmd/frit board` — outstanding plans, who holds each, the agent
- `go run ./cmd/frit open <id>` — focus the pane a plan's lane runs in
- `go run ./cmd/frit nudge <id>` — dry-run the phase prompt; `--go` sends it
- `go run ./cmd/frit claim <id>` — mint frit's atomic hold on a startable plan
- `go run ./cmd/frit start <id>` — compose the rung-3 escalation; `--go` runs it

The discovery verbs are `ready`, `pick`, `next`, `show` and `find`.
Each takes a plan three ways: an exact id, a slug fragment matched
against titles and branches, or nothing at all. The empty form is
inferred from the worktree the command runs in.

They read the fleet index [internal/fleet](internal/fleet) builds, and
reason over it in [internal/discovery](internal/discovery). That
package is pure. The DAG walk, the readiness rule and the selector are
tested against an in-memory fleet with no repository on disk.

## CI and Release

Both workflows live in `.github/workflows` and every action in them
is pinned by commit SHA with the version in a trailing comment.

[ci.yml](.github/workflows/ci.yml) runs the gate this file already
describes — build, vet, test, golangci-lint, `mdsmith check .` — on
every push and pull request to `main`, plus zizmor over the workflows
themselves. It is the local gate run where it cannot be skipped, so a
job that drifts from the Build & Test commands above is the bug.

The markdown job pins the mdsmith action to the same version `go.mod`
imports. Bump the two together, or frit lints with one release and
parses plans with another.

[release.yml](.github/workflows/release.yml) is run from the Actions
"Run workflow" button with a version like `v0.1.0`. A pushed tag is
deliberately not the trigger: a tag is public the moment it exists,
so a failed build would leave one pointing at binaries nobody can
download. Instead the version is validated and the suite run first,
five platform binaries are built with `-X main.version=$VERSION`
stamped in, and the release job creates the tag only once they exist.

Each binary carries a build-provenance attestation, so a download can
be checked against the repository rather than trusted:

```sh
gh attestation verify frit-linux-amd64 -R jeduden/frit
```

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
<?/include?>
