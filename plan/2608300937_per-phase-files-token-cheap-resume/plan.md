---
id: 2608300937
title: Per-phase files give a plan a token-cheap resume bundle
status: "🔳"
summary: >-
  A folder plan keeps every phase inside one plan.md, so a phase read
  parses the whole file and the file grows until the headroom signal
  fires. Let a folder plan carry each phase as its own phase-N.md spec
  and phase-N.result.md living record. The result file is written
  during the phase, not only at its close: a model parks a follow-up or
  side quest there and stays on task, and writes a Handoff section when
  the phase lands. A phase is done when that marker is present, so the
  open phase is the first phase-N.md whose result lacks it — a glob and
  a parse, in a lane or off a ref, needing no frontmatter ledger. A new frit phase
  verb hands a working session its bundle: the open phase's spec, the
  previous phase's handoff, its own parked notes, the tier, the gate,
  and the result file to write. A thin skill fronts it so a cheap model
  runs the loop without guessing. The superseded headroom signal
  retires and the folder shape becomes the default plan-new writes. Flat
  and inline-section plans keep working untouched.
model: sonnet
depends-on: [2608280653]
phases:
  - n: 1
    title: frit phase finds the open phase and emits the working bundle
    status: "✅"
  - n: 2
    title: mdsmith bump and the phase-spec/phase-record kinds
    status: "🔳"
---
# Per-phase files give a plan a token-cheap resume bundle

## Goal

A directory-based plan can carry each phase as its own `phase-N.md`
spec and `phase-N.result.md` living record. The result file holds parked
follow-ups and side quests during the phase and a Handoff section at its
close; a phase is done when that marker is present. A new `frit phase`
verb finds the open phase — the first `phase-N.md` whose result lacks the
marker — by glob, and hands a working session a minimal bundle: that
spec, the previous phase's handoff, its own parked notes, the tier, the
gate, and the result file to write. A session that resumes after its
cache is dropped loads a few small files instead of the whole plan, and a
cheap model runs the loop from the bundle without guessing. No per-phase
frontmatter ledger. The headroom signal retires and `plan-new` writes a
folder plan by default. Flat plans and folder plans that keep phases
inline in `plan.md` are unchanged.

## Context

Today a phase's prose and its status both live in `plan.md`. The
frontmatter `phases:` list carries a per-phase status, and
[attachPhaseBodies](../../internal/planmeta/plan.go) copies each
`## Phase N` section into that phase's `Body`. So a phase read parses the
whole file; `plan.md` grows with every phase until the `headroom` signal
warns it is nearly full; and the status ledger can drift from the work,
which is why plan-phase must flip it inside the same commit that lands
the phase.

**The result file is a living record, and it is the state.** With each
phase in its own file, its state is the filesystem, not frontmatter.
`phase-N.result.md` is written *during* the phase: when a model hits a
follow-up or a side quest it parks it there and stays on the phase,
rather than chasing the tangent — which bloats context — or losing it
when the cache drops. At the close the model writes a `## Handoff`
section: the outcome and what the next phase inherits. A phase is done
when that section is present as a top-level heading. frit finds it with
the same markdown AST walk it uses for `## Goal` and `## Phase N`, not a
raw substring. So a `## Handoff` quoted in a parked note or fenced in a
code block does not count the phase done. The open phase is then the
first `phase-N.md` whose result file is absent or carries no `## Handoff`
heading — a glob and a parse, needing no `phases:` list to stay in sync.
The result file is the work artifact, so the state cannot drift from the
truth the way a hand-flipped ledger can.

**One call, nothing to guess.** The point of the bundle is that a cheap
tier can resume and finish a phase without reading the whole plan. That
holds only if one call hands over every input. So `frit phase <id>` is a
new execution verb, distinct from the discovery family (`next`, `ready`,
`pick`, `board`), that returns the open phase's spec, the previous
phase's `## Handoff`, the open phase's own in-progress notes when it has
any, the tier and gate, and the exact result file to write — the way
[next_action](../2608260639_dispatch-report-carries-next-action.md) already
hands a consumer its next verb. A thin skill fronts the verb (frit's "a
skill fronts every verb" rule), carrying the loop as a fixed recipe
within the 650-token budget, so a small model runs it rather than
reconstructing it.

**Readable both ways.** Finding the open phase works in a lane (list the
plan folder) and off a ref: [collect.go](../../internal/plans/collect.go)
already walks a ref's plan tree over the recursive `ls-tree -r` that
[gitobj](../../internal/gitobj/git.go) runs, so the `phase-N.md` and
`phase-N.result.md` blobs on a ref are enumerable, and the `## Handoff`
heading is parsed from the result blob. The fleet view can report phase
progress, and open follow-ups, from the files alone.

**Folder plans are the enabler.** Plan
[2608280653](../2608280653_folder-based-plans.md) landed
`plan/<id>_<slug>/plan.md` with companion files beside it, and its
[.mdsmith.yml override](../../.mdsmith.yml) already lints every
`plan/*/*.md` but `plan.md` loosely. So `phase-N.md` and
`phase-N.result.md` are accepted companions today — discovery keeps
`plan.md`, drops the rest, and only a `[0-9]*_*.md` name is reported as
mislaid, which `phase-*` never matches. So Phase 1 needs no discovery or
`.mdsmith.yml` change to read the new files: the loose override holds
them.

**First-class lint, by default.** Holding the phase files loosely is
enough to read them, not to make them the shape a plan is written in.
That override — `plan/*/*.md` but `plan.md`, every rule off — is meant
for freeform companions like research and fixtures. A `phase-N.md` spec
and a `phase-N.result.md` record are structured plan content, and a
cheap model loads them, so they earn real rules. A later phase adds two
mdsmith kinds — one for the spec, one for the record — each with a
`required-structure` schema and a token budget like the skill kind's, and
narrows the freeform override to exclude them. The `plan` kind's own
schema loosens in step, since `## Phase N` sections leave `plan.md` for
their files. These land in frit's own [.mdsmith.yml](../../.mdsmith.yml)
and the scaffold default
[frit init ships](../../internal/scaffold/assets/mdsmith.yml), so a fresh
repo lints the default layout with no hand-config. Editing `.mdsmith.yml`
needs consent per [CLAUDE.md](../../CLAUDE.md); it was given for this
plan. That phase first bumps mdsmith to v0.55.1 — go.mod and the CI
action pin together, per [development.md](../../docs/development.md) — so
the new rules are authored against the version CI runs, and 0.55.0's
new rules (MDS074, MET007) and its list-valued `filename:` are on hand.

**The seam.** A phase resolves through
[planmeta.Parse](../../internal/planmeta/plan.go), which takes only the
`plan.md` bytes and so cannot see a sibling file. The open-phase glob and
the bundle assembly sit one level up, where the plan's directory is
known. When the folder holds no `phase-N.md` at all, the verb falls back
to the `plan.md` `phases:` ledger and its `## Phase N` sections, so every
existing flat and inline-section plan works unchanged.

**Reuse first.** In-lane reads already exist: plan
[2608251958](../2608251958_next-show-read-the-held-lane.md) taught the read
verbs to reach the working-tree `plan.md` via
[fleet.CurrentPlanID](../../internal/fleet/current.go) and
[herdr.Resolve](../../internal/herdr), which yield the worktree root.
`frit phase` globs and reads from that same root.

**frit only reads.** The skill writes `phase-N.md` and
`phase-N.result.md`; frit reads them. Writing the `## Handoff` marker is
what completes the phase, so there is no separate status flip to keep
honest. The one-mutation rule is untouched: frit still mints only the
claim.

**Headroom is superseded.** The [headroom signal](../../internal/headroom)
answers "is there room for another `## Phase N` in `plan.md`". With
phases in their own files, `plan.md` does not grow with phases, so the
question loses its force. The remedy for "no room" is now "split the
phases out", a structural fix, not a warning. A later phase retires the
check, points `plan-new` at the layout, and makes the folder shape the
default a plan is authored in — this plan's own file among the first to
move.

## Tasks

1. Add `frit phase <id>`: find the open phase — the first `phase-N.md`
   whose `phase-N.result.md` lacks a `## Handoff` — and emit the working
   bundle (spec, previous handoff, own notes, tier, gate, result path),
   with a fallback to the `plan.md` ledger and sections, and ship the
   thin skill that fronts the verb.
2. Bump mdsmith to v0.55.1; add `phase-spec` and `phase-record` mdsmith
   kinds, each with a required-structure schema and a token budget, for
   `phase-N.md`/`phase-N.result.md`; narrow the freeform companion
   override to exclude them; ship both kinds in frit's own
   `.mdsmith.yml` and the scaffold default.
3. `plan-new` authors a folder plan with `phase-N.md`/`phase-N.result.md`
   by default, and the headroom signal retires.

## Phase 1: frit phase finds the open phase and emits the working bundle

`frit phase <id>`, run inside a plan's own lane, finds the open phase.
That is the first `phase-N.md` whose `phase-N.result.md` is absent or
lacks a `## Handoff` marker. It reports a working bundle. The bundle is
the spec and the previous phase's `## Handoff`. It adds the open phase's
own notes, the tier, the gate, and the result file to write. When the
folder holds no `phase-N.md`, it falls back to the `plan.md` `phases:`
ledger and `## Phase N` sections. This slice proves the verb end to end.
It fixes the report shape and the folder fixture the later phases copy.

RED, at the [cmd/frit](../../cmd/frit) level with a real-repo worktree
fixture, mirroring
[2608251958](../2608251958_next-show-read-the-held-lane.md)'s in-lane test.
Build a repo whose default branch carries a folder plan
`plan/<id>_x/plan.md` — plan-level `status` only, no `phases:` list —
with companions `phase-1.md`, `phase-1.result.md` (carrying a
`## Handoff`), and `phase-2.md` (no result file). Check out a linked
worktree on the plan's work ref. Run `frit phase` with the cwd in that
worktree. Assert the report names Phase 2 as the open phase, carries
`phase-2.md`'s body as the spec, carries `phase-1.result.md`'s handoff,
and names `phase-2.result.md` as the file to write. A second case: the
same plan with a `phase-2.result.md` whose `## Follow-ups` notes quote a
`## Handoff` line inside a code fence, but carry no `## Handoff` heading
of their own, still reports Phase 2 open and carries those notes —
proving the done-test parses headings, not substrings. A third case: a
plan with no `phase-N.md` files reports its open phase from the `plan.md`
ledger and sections, proving the fallback.

GREEN: add a resume assembler beside
[planmeta.Parse](../../internal/planmeta/plan.go). It takes the plan's
directory and globs `phase-*.md`, ordering them numerically by N so
`phase-2` precedes `phase-10`. It returns the first phase whose result
file is absent or carries no `## Handoff` top-level heading — detected by
parsing the result through the mdsmith AST, not a substring match, so a
fenced or quoted `## Handoff` in a parked note does not count. With it
come the spec text, the previous phase's handoff text, and the open
phase's own notes text. When
the glob finds no `phase-N.md`, return the ledger's `FirstOpenPhase` and
its section body. Add the `phase` verb in [cmd/frit](../../cmd/frit), wired
to the in-lane worktree root. Add a bundle report model in
[internal/report](../../internal/report). It carries the open phase, spec,
handoff-in, notes, tier, gate, and result path. Ship the thin skill that
fronts `frit phase` in this same change (frit's "a skill fronts every
verb" rule), carrying the loop as a fixed recipe within the 650-token
budget.

Gate: the three RED cases pass; built `frit phase` inside the fixture
lane names Phase 2, prints its spec and Phase 1's handoff, and names
`phase-2.result.md`; the `phase` skill fronts the verb and stays within
its token budget; `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
stay clean.

## Phase 2: mdsmith bump and the phase-spec/phase-record kinds

`phase-N.md` and `phase-N.result.md` become first-class mdsmith kinds —
`phase-spec` and `phase-record`. Each carries a required-structure
schema pinning the filename, plus a token budget the way the `skill`
kind keeps one. A spec or a record earns real rules this way, instead
of the freeform companion override's blanket "everything off." The
freeform override narrows to exclude both. Both kinds ship in frit's
own `.mdsmith.yml` and in the scaffold default,
[mdsmith.yml](../../internal/scaffold/assets/mdsmith.yml), so a fresh
`frit init` repo lints the layout unconfigured. mdsmith bumps to
v0.55.1 first. go.mod and the CI action pin move together, per
[development.md](../../docs/development.md), so the new kinds are
authored against the version CI runs.

RED: a Go test builds a fixture repo carrying frit's own `.mdsmith.yml`
— copied, the way [doctor_test.go](../../cmd/frit/doctor_test.go)
already does for its own fixtures. The fixture also carries an
oversized `phase-N.md` and `phase-N.result.md` under a folder plan.
Today's config still runs the freeform companion override; neither
`phase-spec` nor `phase-record` exists yet. So mdsmith's own
`Session.Check` reports no `token-budget` diagnostic for either file,
and the test's assertion that one fires fails.

GREEN: add the two kinds to `.mdsmith.yml`. `phase-spec` gets
`path-pattern: "plan/*/phase-*.md"`, a schema pinning
`filename: "phase-*.md"`, `first-line-heading`/`heading-increment`
turned off since a spec stays prose, and a token budget of 800.
`phase-record` gets `path-pattern: "plan/*/phase-*.result.md"`, a
schema pinning `filename: "phase-*.result.md"`, the same two rules off
plus `paragraph-readability`/`paragraph-structure` — its own
`## Handoff` heading opens at level 2 — and a token budget of 900. Add
`kind-assignment` entries assigning them by glob; negate the
`phase-record` glob out of the `phase-spec` one so a result file never
double-matches the spec kind. Narrow the freeform override's glob to
exclude `phase-*.md`. Mirror both new kinds into the scaffold default.
Bump `github.com/jeduden/mdsmith` to v0.55.1 in `go.mod`/`go.sum` and
in the CI action pin and its `version:` input, in
[ci.yml](../../.github/workflows/ci.yml).

Gate: the RED test's fixture now reports `token-budget` for both
oversized files. A well-formed `phase-N.result.md` carrying only a
`## Handoff` heading — the shape
[phase-1.result.md](phase-1.result.md) already is — still lints clean
under the new `phase-record` kind. `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .`, built at v0.55.1, stay clean.

## Execution

Phase 1 is the proving slice: it fixes the folder fixture and the bundle
report shape the later phases reuse.

| Phase                          | Design | Implement | Gate that catches a wrong answer                                         |
| ------------------------------ | ------ | --------- | ------------------------------------------------------------------------ |
| 1 frit phase finds and bundles | opus   | sonnet    | in the lane, phase names phase 2, prints its spec, phase 1 handoff, path |
| 2 mdsmith bump and phase kinds | opus   | sonnet    | mdsmith check . passes; RED fixture's token-budget fires for both kinds  |

## Acceptance Criteria

- [x] Inside a folder plan's own lane, `frit phase` names the first
      `phase-N.md` whose result carries no `## Handoff` heading as the
      open phase and reports its spec
- [x] The done-test parses the result for a `## Handoff` top-level
      heading, so a fenced or quoted mention does not complete a phase
- [x] It carries the previous phase's `## Handoff` and the open phase's
      own in-progress notes, each empty when absent
- [x] It names the result file to write for the open phase
- [x] A plan with no `phase-N.md` files reports its open phase from the
      `plan.md` ledger and sections, unchanged
- [ ] `plan-new` authors a folder plan by default
- [ ] A `phase-N.md` spec and a `phase-N.result.md` record each lint
      under their own mdsmith kind, with a token budget, and the
      freeform companion override no longer covers them
- [ ] The new kinds ship in both frit's `.mdsmith.yml` and the scaffold
      default, so a fresh `frit init` repo lints the layout unconfigured
- [ ] mdsmith is bumped to v0.55.1 in go.mod and the CI action pin
      together, and `mdsmith check .` stays clean
- [x] A thin skill fronts `frit phase`, shipped in the same change as
      the verb
- [x] frit writes no plan file; the skill writes the per-phase files
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
