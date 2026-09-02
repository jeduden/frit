# Development

Build, test and release reference, plus the mechanics behind the
shapes [CLAUDE.md](../CLAUDE.md) and [ux-principles.md](ux-principles.md)
describe.

## Build & test commands

Requires Go 1.25+. Dev tools build from `tools/go.mod` so their
dependency trees never constrain consumers of this module.

- `go build ./...` — build all packages
- `go test ./...` — run all tests
- `go test -run TestName ./...` — run a specific test
- `go test ./... -coverpkg=./... -coverprofile=cover.out` — all tests,
  one coverage profile; `go tool cover -func=cover.out` summarises it,
  `-html=cover.out` browses it
- `go vet ./...` — run go vet
- `go tool -modfile=tools/go.mod golangci-lint run` — lint
- `go mod tidy -modfile=tools/go.mod` — tidy the tools module
- `go run ./cmd/frit repos` — list discovered repos and worktrees
- `go run ./cmd/frit plans` — read plan files off every ref
- `go run ./cmd/frit drift` — per not-done plan: landed?, which commits name it
- `go run ./cmd/frit orphans` — claims, checkouts, rescue refs that disagree
- `go run ./cmd/frit reap` — tear down what orphans reports; `--go` acts
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
- `go run ./cmd/frit yield <id>` — park a fenced lane's work and tear it down
- `go run ./cmd/frit start <id>` — compose the rung-3 escalation; `--go` runs it
- `go run ./cmd/frit skills` — install the bundled agent skills

## The plan catalog and its merge driver

Run `mdsmith merge-driver install` once per clone. `PLAN.md` is a
mdsmith catalog, regenerated from the plan files rather than edited,
so two branches adding a plan conflict on it by construction. The
driver regenerates the catalog during a merge or rebase instead of
leaving conflict markers. frit itself never reads `PLAN.md` — it
enumerates the plan files directly — so this is repo hygiene, not a
frit dependency. `.gitattributes` is committed; the driver is
per-clone local config, which is why each clone installs it.

## Re-recording the JSON golden files

Re-record the golden files with `go test ./internal/report -update`,
and read the diff before committing it. Every consumer of frit is
written against those files.

## The executable scenario matrix

The lease protocol's scenario matrix in
[lease-protocol.md](research/lease-protocol.md) is executable: every
`S<n>` row has a Gherkin scenario tagged `@S<n>` under `features/`,
one file per matrix section, run by godog from `cmd/frit`'s
`TestFeatures`. A scenario still tagged `@pending` is declared but
unwritten, and is skipped rather than run. `internal/scenario` keeps
the two sides in bijection: a row with no tag, a tag with no row, or a
malformed or duplicate id on either side fails `go test ./...`.

- `go test ./cmd/frit -run TestFeatures` — every scenario, pending ones skipped
- `go test ./cmd/frit -run TestFeatures/S16` — one scenario, by its id
- `go test ./internal/scenario` — the matrix/features gate alone

To add a scenario, add its row to the matrix and a tagged scenario to
the section's feature file. Tag it `@pending` until its steps exist.
To write one, drop `@pending` and write its Given/When/Then. Bind the
step functions in the section's own `cmd/frit/bdd_<section>_test.go`,
appended to the step registry from `init` the way
`bdd_lease_test.go` is — a section adds a file, never a line to
`bdd_test.go`, so sections land in any order. godog runs strict, so a
step that matches no definition fails instead of passing as undefined,
and a step text two sections both define fails as ambiguous.

## The skills bundle

frit ships the instructions for driving frit. `frit skills` lays a
suite of Claude Code planning skills into a repository's
`.claude/skills`. A repo frit indexes then carries the workflow an
agent loads to work its plans. The suite is `plan-pick` (find, claim,
start the next lane), `plan-phase` (execute one phase test-first),
`plan-new` (author a plan per `plan/proto.md`), `plan-sync` (reconcile
statuses against `drift` evidence), `plan-tidy` (read `orphans`/`stale`,
act with `yield`/`release`/`reap`, never raw git), and `plan-drive`
(survey with `board`/`who`, drive a lane up the `open`→`nudge`→`start`
ladder). The first five work a plan; `plan-drive` orchestrates from
outside. Health verbs fold into the skill owning that shape: `doctor`'s
checks are what `plan-new` shapes a plan to satisfy, so its call lives
there.

The skills are embedded in the binary from
[internal/skills](../internal/skills), so a shipped frit needs no
companion files. Writing them is scaffolding of the same class as
`frit init`: not the claim, not a mutation of any ref, and it refuses
to clobber an edited skill without `--force`. The canonical text lives
in `internal/skills/assets`; frit's own `.claude/skills` is the output
of `frit skills`, regenerated from the bundle rather than hand-kept, so
it never drifts from what ships.

A shipped skill's commands read `{{frit}}`, substituted by `--via` at
install time. The default is bare `frit`, a binary on `PATH`. A repo
that pins frit with mise, or builds it locally, passes its own. frit's
own `.claude/skills` regenerate with `--via "go run ./cmd/frit"`,
guarded by `TestDogfoodCopiesMatchCanonical` in
[internal/skills/skills_test.go](../internal/skills/skills_test.go): it
fills the token, then fails if a dogfood copy diverges.

The `skill` kind in [.mdsmith.yml](../.mdsmith.yml) keeps every skill
skimmable and token-cheap: readability stays on, a hard line cap keeps
each skill short, and MDS028 `token-budget` caps each at 650 heuristic
tokens. Both the canonical assets and the installed copies are linted.
Uniqueness of the skill `name` is scoped to the assets, because the
dogfooded copies carry those same names by design. It also caps file
and section length, tightens sentence and word caps, and bans filler
prose. `directory-structure` is the one global rule: a stray `.md`
under `internal/skills/assets`, baked into the binary, fails.

## Project layout

Follows the standard Go project layout:

- `cmd/frit/` — main entry point
- `internal/` — private packages
- `plan/` — plan files, indexed by PLAN.md, flat or in a folder with companions

## CI and release

Both workflows live in `.github/workflows` and every action in them
is pinned by commit SHA with the version in a trailing comment.

[ci.yml](../.github/workflows/ci.yml) runs the gate CLAUDE.md
describes — build, vet, test, golangci-lint, `mdsmith check .` — on
every push and pull request to `main`, plus zizmor over the workflows
themselves. It is the local gate run where it cannot be skipped, so a
job that drifts from the Build & test commands above is the bug.

The test job writes one coverage profile across every package —
unit tests and the godog scenarios contribute to the same file, with
`-coverpkg=./...` so a BDD step in `cmd/frit` that drives
`internal/claim` counts toward `internal/claim` — and uploads it as
the `coverage` artifact. It is a measurement, not a gate: nothing
fails on a percentage.

The markdown job pins the mdsmith action to the same version `go.mod`
imports. Bump the two together, or frit lints with one release and
parses plans with another.

[release.yml](../.github/workflows/release.yml) is run from the Actions
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
