---
id: 2608211936
title: A blocked scavenge names its rescue ref and what to do next
status: "🔲"
summary: >-
  A scavenge blocked by an existing rescue ref refuses safely — the
  parked commits and the plan's own ref both stand — but the refusal
  is a dead end: it names the ref, not what to do about it, the
  parked commits are never fetched locally so there is nothing to
  inspect, and `frit start`'s table output drops the message
  entirely. No sweep lists stray rescue refs; only `next` and `show`
  do, one selected plan at a time. Name the conflict as its own
  error, word it as an executable next step on all four verbs that
  park, and have orphans list every leftover rescue ref in one
  listing per repository, before release or claim ever hits the
  refusal.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: a named rescue conflict, worded as a next step
    status: "🔲"
  - n: 2
    title: orphans lists leftover rescue refs first
    status: "🔲"
---
# A blocked scavenge names its rescue ref and what to do next

## Goal

A scavenge blocked by an existing rescue ref tells the agent exactly
which ref and what to do about it, on every verb that can hit the
refusal. `frit orphans` lists leftover rescue refs before anyone
trips over one.

## Context

This repository's own merge method is squash-merge (see the plan
2608211326 PRs). A landed plan's work is essentially never an
ancestor of the default branch because of it.
[`hasUnlanded`](../internal/claim/lease.go) reads it as unlanded
every time, and [`Scavenge`](../internal/claim/lease.go) tries to
park it before deleting the ref.
[`park`](../internal/claim/lease.go) pushes create-only to
`refs/frit/rescue/<id>/<holder>`. The name is per plan and per
machine, so the ref it can collide with is always this same lane's
own earlier park — never another plan's or another machine's work. A
retry at the same tip is an idempotent no-op (`park` recognizes its
own earlier push; `TestScavengeIsIdempotent` pins it). The refusal
fires only when the rescue ref holds a *different* tip: the lane was
re-acquired, one session parked one tip, and a later scavenge or
yield tried to park another:

```text
rescue ref refs/frit/rescue/2608211326/je-framework already holds other work; not deleting plan 2608211326's ref
```

That is the refusal hit live while closing plan 2608211326. `frit
release` correctly declined to clobber the lane's earlier parked
tip. It named the ref, but not what to do next. Rescue refs are
never fetched locally, so there was nothing to inspect without the
right `git fetch` incantation. No sweep had reported the ref
beforehand either. `frit next` and `frit show` do list rescue refs,
but only for one selected plan; `frit orphans` walked right past it.

### Reuse

- [`park`](../internal/claim/lease.go)'s conflict branch already
  names the ref and the plan id; it returns a plain `fmt.Errorf`,
  not a type a caller can match on. Give it a typed error instead of
  hand-rolling a second implementation.
- `park` has two callers, so the error reaches four verbs by two
  routes: [`Scavenge`](../internal/claim/lease.go) feeds
  [`scavengeRef`](../cmd/frit/claim.go) — shared by `claim`,
  `release` and `start` via the `scavengeReporter` interface — and
  [`Yield`](../internal/claim/lease.go) feeds
  [yield.go](../cmd/frit/yield.go)'s own `park:` warning branch.
  Both render through `doc.Warn`, so a better message from `park`
  reaches all four. One rendering hole: `printStart` in
  [start.go](../cmd/frit/start.go) never prints `doc.Warning`, so
  start's table output needs the `warning:` branch `printClaim` and
  `printRelease` already have.
- [`claim.RescueRefs`](../internal/claim/lease.go) lists one plan's
  rescue refs with one `ls-remote` per call; `next` and `show` call
  it once for the selected plan
  ([`rescueRefsFor`](../cmd/frit/main.go)). A sweep needs the same
  data batched: one `ls-remote <remote> "refs/frit/rescue/*"` per
  repository, bucketed by the id path segment — the
  one-process-answers-for-every-ref doctrine
  [`gitobj.RefTimes`](../internal/gitobj/git.go) already states.
- The `orphans` command already derives each repo's landed set:
  `repoLanes` in [cmd/frit/main.go](../cmd/frit/main.go) computes
  `index.LandedIDs` for `lanes.Build` and discards it. Return it
  alongside the lanes rather than computing it a third time — the
  copy inside [`gatherRepo`](../internal/fleet/gather.go) is
  consumed by `heldBranches` and never leaves that function. Note
  `LandedIDs` marks ✅ *and* ⛔ ids, so the report must label a
  superseded plan as superseded, never as landed.
- [`OrphansDoc.AddStale`](../internal/report/orphans.go) is the
  precedent for a category populated after `AddRepo`: copy its
  shape for an `AddRescued`, with membership in `OrphanRepo.Any()`
  and a zero-init in `AddRepo` so a rescue-only repo still renders
  and the JSON keeps the never-null list contract.

## Non-goals

- No change to *when* a scavenge is attempted, or to the staleness
  gate `scavengeGlyph` already applies. This plan only makes a
  blocked attempt legible, and makes the ref it is blocked on
  discoverable first.
- No automatic deletion of a rescue ref. Confirming its content is
  safe to drop is a judgment call this plan leaves to whoever reads
  the refusal or the orphans row — `park`'s rules, refuse rather
  than clobber and same-tip retries stay no-ops, both stand.

## Tasks

1. Phase 1 — `park` returns a typed `RescueConflictError`, worded
   as what to do next; all four parking verbs surface it,
   `printStart` included, and the docs describe the refusal.
2. Phase 2 — `orphans` lists leftover rescue refs in one listing
   per repository, and every doc that scopes `orphans` learns the
   new category.

## Phase 1: a named rescue conflict, worded as a next step

RED, three failing tests:

- extend `TestScavengeRefusesAForeignRescue` in
  [lease_test.go](../internal/claim/lease_test.go) — the
  different-tip scenario is already there (the fixture parks
  `other`, not `tip`); it just never asserts on the shape of the
  error. Add an `errors.As` to a new `RescueConflictError` carrying
  the plan id and the rescue ref. Leave the same-tip case alone:
  `TestScavengeIsIdempotent` pins that a retry at the parked tip is
  a clean no-op, and it must stay green.
- a cmd-level start case: a blocked scavenge during `frit start`
  shows the warning in table output. Today `printStart` drops
  `doc.Warning` on the floor and returns early on `Refused`.
- extend `TestYieldWarnsRatherThanFailsOnAParkConflict` in
  [yield_test.go](../cmd/frit/yield_test.go) to assert the new
  wording reaches yield's warning too.

GREEN: add `RescueConflictError` beside `FenceError` and the other
lease errors in [lease.go](../internal/claim/lease.go), carrying the
plan id and the rescue ref. `park` returns it. Its `Error()` is one
line, operation-neutral so `Yield` can share it honestly, and worded
as the next step:

```text
rescue ref %s holds an earlier park at a different tip; fetch and inspect it, then delete it and retry
```

`Scavenge` wraps it with `%w` to add the one clause it alone knows:
`not deleting plan %d's ref`. `errors.As` still matches through the
wrap. Yield skips that wrap, so it never claims a deletion it does
not perform. `scavengeRef` and yield's branch keep rendering through
`doc.Warn`, unchanged. The one rendering fix is `printStart`. It
gains the `warning:` branch `printClaim` and `printRelease` already
have, on its refused path included.

Docs land in the same commits. The "Landed evidence and scavenge"
section of [docs/claiming.md](../docs/claiming.md) documents the
blocked park and its next step. The closing paragraph's claim that a
manual delete "is not part of the current mechanism" is updated to
admit the one delete the refusal now instructs. The plan-phase skill
gains a line decoding the refusal. Edit it in
[internal/skills/assets](../internal/skills/assets), then regenerate
with `frit skills` — never in `.claude/skills` directly.

Gate: the three RED cases pass; `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: orphans lists leftover rescue refs first

RED: cmd-level cases beside the existing orphans suite. The
migratable and stranded cases live in
[main_test.go](../cmd/frit/main_test.go); the matured-hold case in
[discovery_test.go](../cmd/frit/discovery_test.go). A plan carrying
a rescue ref is reported with its state: landed for ✅, superseded
for ⛔, in-flight otherwise. Under squash-merge every scavenged
landed plan leaves one, so the rows are the cleanup queue. A
superseded plan's parked work must never be labeled landed. A plan
with no rescue ref is not reported. A repo whose only finding is a
rescue ref still renders in the table — that case forces the
`Any()` update. A small case in
[orphans_test.go](../internal/report/orphans_test.go) pins the
`Any()` membership and the zero-init, as the stale-hold cases do.

GREEN: a new `Rescued` category in
[orphans.go](../internal/report/orphans.go) — plan id, state, and
`Refs []string`, the encoding `NextDoc.Rescue` already uses. Add an
`AddRescued` modeled on `AddStale`, membership in
`OrphanRepo.Any()`, and a `[]Rescued{}` zero-init in `AddRepo`.
Populate it in `orphansCmd.Run`, not the shared gather — every
fleet verb runs `gatherRepo`, and only orphans needs this. One
`ls-remote <remote> "refs/frit/rescue/*"` per repository does it,
via a batched sibling of `claim.RescueRefs`, bucketed by id. Join
the buckets with the landed set `repoLanes` already computes —
returned, not recomputed. An unreadable remote records a `Problem`
for the repo rather than silently reporting nothing. Render the
rows in `printOrphans`' three-cell shape. Extend the
`goldenOrphans` fixture so the re-recorded golden pins a
*populated* rescued entry, and read the diff.

Docs land in the same commits.
[docs/claiming.md](../docs/claiming.md)'s orphans category table
names the new row. Its "`frit next` and `frit show` list a plan's
rescue refs" sentence adds `orphans` as the sweep. Its closing
"closed by" list gains this plan. The `orphans` one-liners widen
from "claims and checkouts" in [CLAUDE.md](../CLAUDE.md),
[README.md](../README.md) and the kong help string in
[cmd/frit/main.go](../cmd/frit/main.go).

Gate: the RED cases pass in both renderings; `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run`,
`mdsmith check .` and the golden diff are clean.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                                                  |
| ------------------ | ------ | --------- | --------------------------------------------------------------------------------- |
| 1 named conflict   | opus   | sonnet    | test: a different-tip rescue's refusal is `RescueConflictError` on all four verbs |
| 2 orphans finds it | opus   | sonnet    | test: a rescue ref is reported with its plan's state; a rescue-only repo renders  |

## Acceptance Criteria

- [ ] A park blocked by an earlier park at a different tip returns a
      typed `RescueConflictError` naming the plan id and the ref,
      worded as what to do next; a same-tip retry stays a no-op.
- [ ] The fix lives in `park`: `claim`, `release`, `start` and
      `yield` inherit the wording with no per-verb error shaping,
      and `printStart` renders the warning it used to drop.
- [ ] `frit orphans` lists leftover rescue refs with their plan's
      state — never calling superseded work landed — in one
      `ls-remote` per repository, before anyone triggers a blocked
      scavenge.
- [ ] Every doc describing the surface is updated:
      [docs/claiming.md](../docs/claiming.md) (scavenge refusal,
      category table, rescue-refs sentence, closing list), the
      `orphans` one-liners in [CLAUDE.md](../CLAUDE.md),
      [README.md](../README.md) and the kong help, and the
      plan-phase skill via its asset plus `frit skills`.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
