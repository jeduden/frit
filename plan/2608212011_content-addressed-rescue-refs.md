---
id: 2608212011
title: The rescue ref carries its tip, so a park never conflicts
status: "🔳"
summary: >-
  A rescue ref is named per plan and per machine, so one lane parking
  twice at different tips collides with itself, and park must refuse
  to avoid clobbering its own earlier work. Plan 2608211936 words
  that refusal; this plan removes the conflict class. Naming the ref
  by its content — the parked tip joins the name — makes every park
  land under a fresh name, create-only and conflict-free, with the
  refusal kept only as a trust-domain guard. Then the scenario docs
  are updated: the protocol note's ref shape and PARK rows, and
  claiming.md's yield and scavenge sections.
model: sonnet
depends-on: [2608211936]
phases:
  - n: 1
    title: the parked tip joins the rescue ref name
    status: "🔲"
  - n: 2
    title: the scenario docs learn the new shape
    status: "🔲"
---
# The rescue ref carries its tip, so a park never conflicts

## Goal

Two parks from one lane at different tips both land, each under its
own ref, so the cooperative rescue conflict stops existing. The
scenario docs describe the shipped ref shape.

## Context

[`rescueRef`](../internal/claim/lease.go) names a park
`refs/frit/rescue/<id>/<holder>` — per plan and per machine. One
lane parking twice at different tips — a re-acquired lease scavenged
again, a yield after an earlier scavenge — collides with its own
earlier park. [`park`](../internal/claim/lease.go) then refuses
rather than clobber, and plan 2608211936 makes that refusal legible.
Legible is still a dead end an agent must resolve by hand.

Content-addressing removes the class. With the parked tip in the
name — `refs/frit/rescue/<id>/<holder>/<tip>` — a different tip is a
different name, and the create-only push just lands. The same name
can only hold the same commit, so a retry is a no-op by
construction. The one conflict left is a same-name ref holding a
different object: a forged or hand-moved ref, inside the trust
domain the protocol already assigns to raw write access (S37–S39,
S69). `RescueConflictError` from plan 2608211936 stays for exactly
that guard, reworded — which is why this plan depends on it rather
than racing it through the same lines.

Accumulation is the accepted cost. A lane can now leave several
rescue refs; that is the point — none of them is lost — and the
orphans sweep plan 2608211936 adds lists them all with their plan's
state. Deleting them stays a human judgment. Plan 2608212010
independently stops landed content being parked at all, which bounds
the accumulation to real divergence.

### Reuse

- [`rescueRef`](../internal/claim/lease.go) is the one place the
  name is minted; `Scavenge` and `Yield` both reach `park` through
  it. Adding the tip parameter changes no caller outside
  [lease.go](../internal/claim/lease.go).
- [`claim.RescueRefs`](../internal/claim/lease.go) lists by the
  prefix pattern `refs/frit/rescue/<id>/*`. `git ls-remote` glob
  patterns cross path segments, so the pattern should match both
  the legacy two-segment shape and the new three-segment one — a
  RED case must prove it rather than assume it.
- `TestScavengeIsIdempotent` and
  `TestScavengeRefusesAForeignRescue` in
  [lease_test.go](../internal/claim/lease_test.go) pin today's
  same-tip no-op and different-object refusal; both adapt rather
  than duplicate.

## Non-goals

- No migration of legacy rescue refs. Existing two-segment refs
  stay where they are, keep listing in `RescueRefs` and the orphans
  sweep, and retire by human deletion.
- No cap or cleanup policy on accumulated rescue refs. Listing them
  is the orphans sweep's job; bounding them is plan 2608212010's.
- No change to when parks happen — scavenge and yield park exactly
  as before, only under new names.

## Tasks

1. Phase 1 — the tip joins the rescue ref name; parks stop
   conflicting; the refusal narrows to the trust-domain guard.
2. Phase 2 — the protocol note's ref shape and PARK rows, and
   claiming.md's yield and scavenge sections, describe the shape.

## Phase 1: the parked tip joins the rescue ref name

RED, in [lease_test.go](../internal/claim/lease_test.go) and
[yield_test.go](../cmd/frit/yield_test.go):

- One lane parks two different tips for one plan; both pushes land,
  under two refs, and no refusal fires.
- A park retried at the same tip is a no-op: the ref already holds
  exactly that commit.
- `RescueRefs` lists a plan's legacy two-segment ref and a new
  three-segment ref in one call.
- A same-name ref holding a different object still refuses with
  `RescueConflictError`, reworded for what it now means: the ref
  was moved by hand, frit does not contend for it (adapting
  `TestScavengeRefusesAForeignRescue`'s fixture to plant the forged
  name).
- Yield parks the fenced lane's divergence under the new shape, and
  its report names the full ref.

GREEN: `rescueRef` takes the tip and appends it as the third
segment, full 40-hex so two parks can never alias. `park`'s
conflict branch survives as the trust-domain guard with the
narrowed wording. `Scavenge`'s `%w` wrap from plan 2608211936 is
unchanged. Nothing outside
[lease.go](../internal/claim/lease.go) changes behavior; reports
and listings carry whatever ref names they are handed.

Gate: the five RED cases pass; `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: the scenario docs learn the new shape

The scenario record is
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md);
research notes carry dated inline notes when a decision moves.

RED: a sweep for every claim phase 1 falsified, each finding an
edit. Known already:

- The Scavenge section names the shape
  `refs/frit/rescue/<id>/<machine-id>`; it gains the tip segment
  under a dated note naming this plan, and says why: a park can
  then never contend with an earlier one.
- Matrix rows S40 and S9 and traps F4/F5 cite PARK; their
  mechanism cells are re-read and adjusted where they describe the
  old one-ref-per-machine shape.
- [docs/claiming.md](../docs/claiming.md)'s "Fencing and yield"
  section names the old shape; it and the "Landed evidence and
  scavenge" section describe the new one. The next-step wording
  plan 2608211936 added for the conflict narrows to the forged-ref
  case, since the cooperative conflict no longer exists.
- The closing "closed by" list of claiming.md gains this plan.

GREEN: the edits above, and nothing else — a doc claim the sweep
finds beyond them becomes its own edit in the same commit.

Gate: `mdsmith check .` clean; no doc names a rescue shape or a
park conflict the code no longer has.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase           | Design | Implement | Gate that catches a wrong answer                                     |
| --------------- | ------ | --------- | -------------------------------------------------------------------- |
| 1 tip in name   | opus   | sonnet    | test: two parks at two tips both land; same-tip retry stays a no-op  |
| 2 scenario docs | sonnet | sonnet    | no doc names the old shape or the retired cooperative conflict class |

## Acceptance Criteria

- [ ] One lane parks two tips for one plan with no refusal; each is
      its own ref; a same-tip retry is a no-op.
- [ ] `RescueRefs` lists legacy and new shapes in one call, so no
      parked work goes invisible during the transition.
- [ ] `RescueConflictError` fires only for a same-name ref holding
      a different object, worded as the trust-domain guard it now
      is.
- [ ] The protocol note's Scavenge section and PARK rows, and
      claiming.md's yield and scavenge sections, describe the
      shipped shape under a dated note.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
