---
id: 2608251958
title: next and show read the held lane's own plan, not the default branch
status: "🔲"
summary: >-
  next and show report a plan off the fleet index, whose authoritative
  version is the default-branch copy. Run inside the plan's own held
  lane before its work has merged, they show the merged status: a phase
  the lane already closed still reads as open. plan-phase papers over
  it — "trust the lane's own frontmatter over next". Close the gap:
  when the cwd is the resolved plan's own lane, next and show read the
  working-tree copy, so the executor sees its own lane's status. The
  fleet's default-branch-authoritative rule stays untouched, because
  orphans, claim, ready and pick depend on it. Then retire the caveat.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: next reads the working-tree plan inside its own lane
    status: "🔲"
  - n: 2
    title: show reads it too, and --json names the source
    status: "🔲"
  - n: 3
    title: Retire the plan-phase caveat the gap forced
    status: "🔲"
---
# next and show read the held lane's own plan, not the default branch

## Goal

Run inside a plan's own held lane, `next` and `show` report that lane's
own plan status. A phase the lane closed but has not merged reads as
done, not open. The fleet verbs keep reporting the default-branch
status unchanged.

## Context

The split we hold: the binary indexes and displays; the skill judges.
This is a case where the binary displays the wrong copy, and the skill
was written to work around it.

**The mechanism.** The index keeps one version per distinct blob across
all refs. [rank](../internal/index/index.go) makes the default-ref copy
authoritative. That is deliberate and correct for a fleet view: the
merged status is the shared truth, and
[LandedIDs](../internal/index/index.go), `orphans`, `claim`, `ready`
and `pick` all depend on "default branch = canonical". This plan does
not change `rank`.

**The gap.** [next and show](../cmd/frit/main.go) are execution verbs,
normally run standing in the plan's own lane. There the executor wants
this lane's view. Instead they inherit the fleet's default-branch
version, so a phase closed but unmerged reads as open.
[plan-phase](../internal/skills/assets/plan-phase/SKILL.md) papers over
it: "next and show read the default branch's copy ... trust the lane's
own frontmatter over next there." That caveat is the tell of a binary
gap, and this plan closes it.

**Reuse first.** The cwd-to-lane machinery exists.
[fleet.CurrentPlanID](../internal/fleet/current.go) maps the cwd to its
`(repo, id)` through [herdr.Resolve](../internal/herdr), which yields
the worktree root and branch, and a holds match. `resolveSelector`
already calls it for an empty selector. This plan reuses it to detect
that the cwd is the resolved plan's own lane, and reuses
[planmeta.Parse](../internal/planmeta) to re-read the working-tree
file. The plan directory resolves the same way the walk resolves it,
from the repository's [.frit.yml](../internal/repocfg).

**Scoped, still read-only.** Only `next` and `show` change, and only
when the cwd is the resolved plan's own lane; every other invocation,
and every other verb, reads the fleet version. Nothing is mutated —
the fix picks a different blob to read, not a different thing to write.

**Visible in JSON.** A consumer must be able to tell which copy a row
came from, so the `--json` document names the source (the held lane or
the default branch). The [JSON contract](../CLAUDE.md) keeps every key
present; the goldens in [testdata](../internal/report/testdata) pin it.

## Tasks

1. `next`, run inside a plan's own lane, reads the working-tree copy
   and reports that lane's own status and phases.
2. (determined after Phase 1)
3. (determined after Phase 1)

## Phase 1: next reads the working-tree plan inside its own lane

`next` must prefer the working-tree copy of the plan when the cwd is
that plan's own held lane. This slice proves the detection and the
re-read end to end; the report shape it lands is what Phase 2 copies.

RED, at the [cmd/frit](../cmd/frit) level, with a real-repo worktree
fixture. Build a repo whose default branch carries a plan with Phase 1
at `🔳` (open), and a linked worktree checked out on the plan's work
ref whose working-tree copy has Phase 1 at `✅`. Run `next` with the
cwd set to that worktree. Assert it reports Phase 2 — the next open
phase in the lane's own copy — not Phase 1, which is stale-open only on
the default branch. A second case: cwd outside any lane still reports
the default-branch version, unchanged.

GREEN: after `resolveSelector`, detect the in-lane case —
`fleet.CurrentPlanID(cwd)` equals the resolved plan's `(repo, id)`.
When it holds, read the plan file from the worktree root (via
`herdr.Resolve` for the root and the repo's plan directory), parse it
with `planmeta.Parse`, and override the resolved plan's `Status` and
`Phases` from the local copy before building the report. A missing or
unparsable local file falls back to the fleet version.

Gate: both RED cases pass; `frit next` inside the lane reports the
lane's next open phase; `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: show reads it too, and --json names the source

`show` shares the staleness and the fix; and the divergence must be
legible to a `--json` consumer.

RED, at the [cmd/frit](../cmd/frit) and
[internal/report](../internal/report) levels:

- `show`, run inside the plan's own lane, reports the working-tree
  copy's Goal and status, mirroring Phase 1's case for `next`.
- The `next` and `show` `--json` documents carry a `source` field
  naming where the reported version came from: the held lane or the
  default branch. The goldens in
  [testdata](../internal/report/testdata) — `next.json`, `show.json`,
  and a new lane-sourced pair — pin it, keys always present.

GREEN: apply Phase 1's in-lane override in `show`. Add the `source`
field to the `next` and `show` report models. Record the goldens with
`go test ./internal/report -update`, then read the diff.

Gate: the RED cases pass; the goldens match; `go test ./...`,
`go vet ./...`, `golangci-lint run` and `mdsmith check .` stay clean.

## Phase 3: Retire the plan-phase caveat the gap forced

With `next` and `show` reading the lane's own copy, the plan-phase
caveat is obsolete and must go, so the skill stops teaching a
workaround for a fixed bug.

RED, at [internal/skills](../internal/skills): a test asserts the
plan-phase asset no longer tells the reader to trust the lane's
frontmatter over `next`. This mirrors the existing content guards.

GREEN: rewrite plan-phase step 1 to drop the default-branch caveat,
keeping the rest of the step. Regenerate the dogfood copies
(`frit skills --via "go run ./cmd/frit" --force .`). If any prose in
[CLAUDE.md](../CLAUDE.md) states that read verbs report the
default-branch copy, refine it to name the lane exception, then run
`mdsmith fix AGENTS.md`.

Gate: `frit next` in the Phase 1 fixture reports the lane's own status,
confirming against the built binary the claim plan-phase now leaves
unqualified. The new skills test passes; `TestDogfoodCopiesMatchCanonical`
stays green; plan-phase stays under the 650-token budget;
`go test ./...`, `golangci-lint run` and `mdsmith check .` stay clean.

## Execution

Phase 1 proves the in-lane detection and re-read — the load-bearing
slice. Phase 2 extends it to `show` and makes the source legible in
JSON. Phase 3 retires the caveat the gap forced.

| Phase                          | Design | Implement | Gate that catches a wrong answer                                    |
| ------------------------------ | ------ | --------- | ------------------------------------------------------------------- |
| 1 next reads the lane copy     | opus   | sonnet    | inside the lane, next reports the lane's next open phase, not stale |
| 2 show + --json source, golden | opus   | sonnet    | show reads the lane copy; the source-carrying JSON goldens match    |
| 3 retire the caveat            | opus   | sonnet    | plan-phase no longer teaches the workaround; dogfood stays green    |

## Acceptance Criteria

- [ ] Inside a plan's own lane, `next` and `show` report the
      working-tree copy's status and phases
- [ ] A phase closed in the lane but unmerged reads as done, not open
- [ ] Outside a lane, and for every other verb, the default-branch
      version is reported unchanged
- [ ] `rank`'s default-branch-authoritative rule is untouched
- [ ] `--json` names the source of the reported version and is pinned
      by goldens
- [ ] The plan-phase caveat is gone and the dogfood copies match
- [ ] plan-phase's lane claim is verified against the built binary,
      not only linted
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
