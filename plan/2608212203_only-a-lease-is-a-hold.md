---
id: 2608212203
title: Only a minted lease is a hold, and a dead session frees it
status: "🔳"
summary: >-
  Two tooling gaps let a dead pane strand a plan. frit reads any ref
  matching the holds patterns as a live claim, so a hand-made
  plan/<id> branch with no marker blocks the plan it names. And a
  hold whose bound session herdr confirms gone waits out the whole
  takeover window before any verb will touch it. Close both: count a
  hold only when it carries frit's own claim marker, and make a
  confirmed-dead session free its plan at once (S76).
model: sonnet
depends-on: []
phases:
  - n: 1
    title: a hold is a minted lease, not a name match
    status: "✅"
  - n: 2
    title: a confirmed-dead session frees its plan now
    status: "🔲"
---
# Only a minted lease is a hold, and a dead session frees it

## Goal

A plan reads as held only when a ref carries frit's own claim marker.
A hold whose bound session herdr confirms gone frees its plan at once.
So a dead pane never strands a plan behind a verb that refuses.

## Context

This is the prevention behind the deserted-hold recovery in plan
2608212346. That plan is the safety net; this one removes the two
tooling gaps that create the trap.

The first gap is classification. [gather.go](../internal/fleet/gather.go)
marks a plan `Held` when a ref's name matches the `holds` patterns.
The only marker it consults is `claim.Released`, which drops a ref
whose tip is a release marker. Nothing requires the ref to carry the
claim marker frit mints. So a `plan/<id>` branch made by hand — plan
files committed straight on the base, no marker — reads as a live
hold and blocks the plan. That is exactly how the authoring branch
`plan/2608211936` stranded its plan.

The marker machinery already exists. `claim.MarkerHost` in
[claim.go](../internal/claim/claim.go) reads a ref's marker, and
`claim.Released` in [lease.go](../internal/claim/lease.go) already
walks a ref to classify its markers. Phase 1 reuses them: the held
gate additionally requires a claim or takeover marker in the ref's
history, not just a matching name.

The second gap is latency. A held plan is only a `pick`/`claim`
candidate once its takeover window has matured — `candidate` in
[ready.go](../internal/discovery/ready.go) gates on `p.Stale`. The
window exists because a quiet tip is ambiguous: a slow agent looks
dead. But herdr can report a bound session positively gone, and that
is certain. `herdr.SessionLive` already drives the veto in
[claim.go](../cmd/frit/claim.go). Phase 2 reuses its inverse: a
confirmed-gone session frees the plan with no window.

## Tasks

1. Gate the held classification on a minted claim marker.
2. Make a confirmed-dead bound session free its plan without the
   window.

## Phase 1: a hold is a minted lease, not a name match

A plan reads as held only when a ref matching the `holds` patterns
also carries frit's own claim or takeover marker somewhere in its
history since the base. A pattern-matching branch with no marker is
not a hold. A released ref stays not-held, as today. A legacy
decorated hold still carries a marker, so it still reads as held.

RED, against the fixture idiom the fleet tests use:

- A `plan/<id>` branch of plain commits, no marker: the plan is not
  held, so it is startable.
- A ref carrying a claim marker, tip a later work commit: held.
- A ref whose only commit is the claim marker: held.
- A ref whose tip is a release marker: not held, unchanged.
- A legacy decorated hold: still held, so the migration path is not
  broken.

GREEN: the held fold in [gather.go](../internal/fleet/gather.go)
requires a claim or takeover marker in the ref, read through the
existing [claim](../internal/claim) marker helpers. No new parser.

Gate: the five RED cases pass; `go test ./...` and `mdsmith check .`
are clean; the report golden files still hold or are re-recorded with
the diff read.

## Phase 2: a confirmed-dead session frees its plan now

A held plan whose bound session herdr reports positively gone is a
`pick`/`claim` candidate at once, with no staleness window. A live
session still vetoes. herdr unreachable is not a death, so it falls
back to the window exactly as today.

RED, with the fake-herdr idiom the `who` and claim tests use:

- Held, bound session confirmed gone, window not matured: a
  candidate; `claim` takes it.
- Held, bound session live: not a candidate; the veto holds.
- Held, herdr unreachable: falls back to the window rule, unchanged.

GREEN: the confirmed-gone signal reaches the candidate decision in
[ready.go](../internal/discovery/ready.go) and the take path in
[claim.go](../cmd/frit/claim.go), beside the existing veto and
window checks. The verb-state table gains no silent cell.

Gate: the three RED cases pass; a live holder is never taken over; no
cross-machine clock is consulted; `go test ./...` is clean.

## Execution

Tier is per phase, set by the most demanding ingredient. The design
is settled here and in the protocol note, so both phases implement
from written assertions.

| Phase                 | Design | Implement | Gate that catches a wrong answer                                 |
| --------------------- | ------ | --------- | ---------------------------------------------------------------- |
| 1 marker-gated hold   | opus   | sonnet    | markerless branch is startable; real and legacy leases stay held |
| 2 confirmed-dead free | opus   | sonnet    | confirmed-gone frees now; live vetoes; unreachable uses window   |

## Acceptance Criteria

- [ ] A markerless `plan/<id>` branch does not read as a hold
- [ ] A frit-minted lease, at the marker or ahead of it, still holds
- [ ] A legacy decorated hold still reads as held
- [ ] A confirmed-dead bound session frees its plan with no window
- [ ] A live bound session is never taken over
- [ ] herdr unreachable falls back to the staleness window
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
