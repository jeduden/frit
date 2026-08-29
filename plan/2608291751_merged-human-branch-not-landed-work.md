---
id: 2608291751
title: A merged human plan/<id> branch is not read as landed lease work
status: "✅"
summary: >-
  `frit claim` reads a merged, human-authored `plan/<id>` branch — the
  branch a plan file was written and PR-merged on — as the plan's own
  lease work. `latestMarker` greps `^plan <id>: ` and parses the
  plan-authoring commit `plan <id>: <title>` as a marker, so `heldError`
  reports `Landed`, and `claim` both scavenges the branch and advises
  "set plan <id> to ✅" for a plan whose work never landed. Separately, a
  worktree left on `plan/<id>` at a foreign path reads as a healthy
  staffed lane — it has a hold and a checkout — so `orphans` and `stale`
  never name the checkout that blocks `start`. Two edges follow in the
  same sequence: a `claim` whose stand-up fails keeps a hold no verb can
  end, and a `start` resumed from its own lane re-runs `worktree create`.
  Gate the marker parse on a real marker kind, surface the foreign
  checkout, unwind a failed claim stand-up, and skip creation on resume.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: a plan-authoring commit is not a lease marker
    status: "✅"
  - n: 2
    title: a foreign checkout on the plan branch is an orphan
    status: "✅"
  - n: 3
    title: a claim whose stand-up fails releases what it minted
    status: "✅"
  - n: 4
    title: a start resumed from its own lane skips worktree create
    status: "✅"
---
# A merged human plan/<id> branch is not read as landed lease work

## Goal

A merged, human-authored `plan/<id>` branch is not read as the plan's
own landed lease work. So `frit claim` neither scavenges it nor advises
marking an unstarted plan ✅. And a worktree left on `plan/<id>` off the
lane's path is named by a teardown verb, not left to block `start`.

## Context

Issue [#104](https://github.com/jeduden/frit/issues/104), against
`b8340d2` (0.6.0). The plan file for a plan is authored on a branch
named `plan/<id>` and merged to `main` by PR — the same branch name
frit mints for the lease work ref. Two independent reads then confuse
the human branch with frit's own.

**Gap 1 — the marker parse is too loose.** `latestMarker` in
[lease.go](../internal/claim/lease.go) greps `^plan <id>: ` and hands
whatever it finds to `parseMarker`, which accepts any non-empty kind.
The plan-authoring subject `plan <id>: <title>` matches, so it parses as
a known marker. `heldError` only computes `Landed` once `fetchedMarker`
(→ `latestMarker`) returns ok; with the loose parse it does, and
`landedTip`'s ancestry branch is true for a merged branch, so
`held.Landed` is set. `claim` then renders "the claim branch has already
landed … set plan <id> to ✅" ([claim.go:465](../cmd/frit/claim.go)) and
runs `scavengeLanded` ([claim.go:289](../cmd/frit/claim.go)), deleting
the remote branch. No work had landed — only the plan file.

`terminalMarkerKind` already tells a real marker subject from an
arbitrary one, tolerating the legacy decorated `claim <slug>` a bare
`claim` prefix carries. `markerSubject` is deliberately stricter — it
matches only the four exact kinds, because it drives the delete path
where dropping a decorated subject is not provably safe, so it must not
be the model the parse copies. Reuse `terminalMarkerKind`'s knowledge,
not a second parser, and leave `markerSubject` untouched. Gate
`parseMarker` so a kind that is not one of frit's marker kinds is
rejected. A plan title then never reads as a marker anywhere
`parseMarker` is reached — `latestMarker` and `commitMarker`.

**Gap 2 — a foreign checkout hides as a healthy lane.** After gap 1 is
worked around by hand, `start` re-mints a claim, so the lane in
[lanes.go](../internal/lanes/lanes.go) carries both a hold and the
worktree the human left on `plan/<id>`. `Unstaffed` (hold, no checkout)
and `Stranded` (checkout, no hold) both miss it: it has both. But that
checkout sits at a foreign path, not the lane's own, so
`herdr worktree create` fails with `'plan/<id>' is already used by
worktree at <other path>` and no teardown verb can see it. The claim
marker already records the lane path it was cut for (`leaseMessage`'s
`lane:` trailer). A checkout on the plan's branch whose path is not that
recorded lane path is the foreign one — a signal already in the marker,
not a new lookup.

**Gap 3 — a failed claim stand-up leaves a hold no verb can end.**
`standUpClaimWorktree` in [claim.go](../cmd/frit/claim.go) treats a
failed `herdr worktree create` as a warning and keeps the lease. But
`persistToken` in [token.go](../internal/claim/token.go) writes the
token into the lane's own git dir, and that lane was never stood up.
So no lane holds the token, and `ownToken` fails everywhere. `release`
refuses as "held live by another lane", `yield` as "still held by this
lane", `start` as "not takeable until the window matures". The only
exit is a two-hour takeover window on a hold this host minted seconds
earlier. `start` already handles the same failure right: `releaseLease`
in [start.go](../cmd/frit/start.go) pushes a release marker on a failed
handoff. `claim` should unwind the same way, so the two rungs agree.

**Gap 4 — a resumed start re-creates the worktree.** `startAcquire` in
[start.go](../cmd/frit/start.go) takes the `claim.Resume` path when a
persisted token matches — a start run from inside its own lane. But
`standUpLane` then calls `herdr.WorktreeCreate` unconditionally. It
fails with `'<path>' already exists`, and the unwind releases a lease
that was healthy. A resume already knows it is inside the lane; the
pane it should drive is the current one, which `herdr.CurrentPane`
already reads for `currentSession`. Skip creation on resume and start
the agent in the pane the caller sits in.

## Tasks

1. Gate `parseMarker` on a real marker kind, so a plan-authoring commit
   is never a lease marker and `claim` on a merged human branch refuses
   as a lost race without scavenging or the ✅ advice.
2. (determined after Phase 1) Surface a checkout on the plan's branch
   sitting off the recorded lane path as an orphan, reachable by `reap`.
3. Release the lease `claim` just minted when its worktree stand-up
   fails, the unwind `start` already runs, so no verb-proof hold is left.
4. Skip `worktree create` when `start` resumes from inside its own lane,
   and drive the agent in the current pane.

## Phase 1: a plan-authoring commit is not a lease marker

`parseMarker` in [lease.go](../internal/claim/lease.go) gains a gate. A
body whose subject kind is not one of frit's marker kinds is not a
marker. The accepted kinds are what `terminalMarkerKind` and the beat
renewal already recognise: `claim`, `beat`, `release`, `takeover`, and
the legacy decorated `claim <slug>` (prefix `claim ` tolerated). They
are factored into a `markerKind` helper the parse and `terminalMarkerKind`
share, so there is one list. A plan-authoring subject
`plan <id>: <title>` fails the gate, and `parseMarker` returns ok false.

RED, a focused unit test in
[lease_test.go](../internal/claim/lease_test.go):

- `parseMarker(id, "plan <id>: <title>\n\n…")` returns ok false, where
  `<title>` is not a marker kind.
- Every genuine marker still parses: a claim, a beat, a release, a
  takeover, and a legacy decorated `claim <slug>` body all return ok
  true with the right `Kind` and trailers.
- A claim-protocol test: `heldError` on a tip whose only commit is the
  plan-authoring commit, merged into base, returns `Known` false and
  `Landed` false — so `claim` would refuse as a lost race, never
  scavenge, never advise ✅.

GREEN: add `markerKind(subject, planID) (string, bool)`. It reuses
`terminalMarkerKind`'s tolerance plus `beat`. Call it from `parseMarker`
before the `Marker` is built, returning ok false when it fails. Have
`terminalMarkerKind` read through the same helper, so the kind list is
not duplicated.

Gate: the unit and claim-protocol tests pass; `go test ./...` and
`mdsmith check .` are clean. Then build frit and run `frit claim <id>`
against a fixture repo whose `plan/<id>` branch is a merged
plan-authoring commit; confirm the output is a lost-race refusal with no
`scavenged:` line and no "set plan <id> to ✅".

## Phase 2: a foreign checkout on the plan branch is an orphan

A lane holding a worktree on the plan's branch whose path is not the
lane path recorded in the claim marker is reported by `orphans` and
removable by `reap`. The recorded path rides on the hold: `lanes.Hold`
carries the marker's `lane:` trailer, and `lanes.Find` names a worktree
whose `Path` differs from every hold's recorded lane as a new orphan
kind (a foreign checkout), distinct from `Unstaffed` and `Stranded`.
`reap` gains the teardown for it.

RED first, to pin the real shape against the observed behaviour:

- A `lanes.Find` test: a lane with a live hold recording lane path
  `A` and a worktree checked out on the branch at path `B` is reported
  as a foreign checkout, not swallowed as healthy.
- An `orphans` report/golden test carrying that lane, and a `reap` test
  that tears the foreign checkout down under `--go`.

GREEN sites are settled once the RED reproduction fixes the exact
plumbing. That is: the marker `lane:` threaded onto `lanes.Hold`, the
new `Find` branch, the `OrphansDoc` field and its golden, and the `reap`
action. The [plan-tidy](../.claude/skills/plan-tidy/SKILL.md) skill
fronts these verbs. It is updated in the same phase if the reported
shape changes what it tells an agent to run.

Gate: the `lanes.Find`, `orphans` and `reap` tests pass; `orphans`
goldens re-recorded with `go test ./internal/report -update` and the
diff read; `go test ./...` and `mdsmith check .` clean. Then build frit
and, against a fixture with a worktree on `plan/<id>` at a foreign path,
confirm `frit orphans` names it and `frit reap --go` removes it.

## Phase 3: a claim whose stand-up fails releases what it minted

When `standUpClaimWorktree` in [claim.go](../cmd/frit/claim.go) fails
after a fresh mint, `claim` releases the lease it just pushed — a
release marker, never a delete — and reports the refusal with the
stand-up cause. It no longer keeps a hold with no token behind it. The
unwind is the one `start` already runs: `releaseLease` in
[start.go](../cmd/frit/start.go), lifted so both rungs call one helper
taking `fleet.Coord` rather than `startContext`.

RED, a `claim` command test with a fake herdr whose `worktree.create`
fails:

- After the run, the remote `plan/<id>` tip is a release marker, and
  the doc carries the refusal naming the stand-up cause, not
  `claimed: true` with a warning.
- A following `frit claim <id>` acquires at epoch 2 — the hold was
  ended, not abandoned.
- A stand-up that succeeds still reports `claimed: true` and `stood`
  with the lane path, so the happy path is unchanged.

GREEN: move `releaseLease` to a shared site both `claim.go` and
`start.go` reach. Call it from `standUpClaimWorktree`'s failure branch
with the minted tip. Turn the warning into a refusal.

Gate: the command tests pass; `go test ./...` and `mdsmith check .`
clean. Then build frit and, against a fixture whose lane path is
already occupied, run `frit claim <id>`; confirm the refusal, and that
`frit claim <id>` again succeeds at once with no takeover window.

## Phase 4: a start resumed from its own lane skips worktree create

When `startAcquire` takes the resume path — a persisted token matched,
so the caller sits inside its own lane — `standUpLane` skips
`herdr.WorktreeCreate`. The pane to drive is the current one, read
with `herdr.CurrentPane` the way `currentSession` already does. The
agent start, the prompt and the focus proceed against that pane. A
fresh acquire is unchanged: it still creates the worktree.

RED, a `start` command test with a fake herdr:

- Given a persisted token that resumes, `worktree.create` is never
  called, the agent starts in the current pane, and the lease ends on
  a beat, not a release.
- Given no token, `worktree.create` is called exactly as before.
- Given a resume where `herdr.CurrentPane` fails, `start` refuses with
  the cause rather than falling through to a create that will fail.

GREEN: thread a `resume bool` (or the pane) from `startExecute` into
`standUpLane`. Branch on it before `WorktreeCreate`. Keep the unwind
path for the create branch only.

Gate: the command tests pass; `go test ./...` and `mdsmith check .`
clean. Then build frit, stand a lane up by hand with herdr, persist
the token, and run `frit start <id> --go` from inside it; confirm the
agent starts there and the lease was renewed, not released.

## Execution

Tier is per phase. Both phases are claim- and teardown-protocol
correctness: opus settles the shape against subtle git evidence, sonnet
implements from the written assertions, and a loud gate — a claim
refusal run against the built binary, a re-read golden — catches a wrong
answer.

| Phase              | Design | Implement | Gate that catches a wrong answer                                             |
| ------------------ | ------ | --------- | ---------------------------------------------------------------------------- |
| 1 strict marker    | opus   | sonnet    | claim on a merged plan-authoring branch refuses as lost race, no scavenge/✅ |
| 2 foreign checkout | opus   | sonnet    | orphans names a checkout off the recorded lane path; reap --go removes it    |
| 3 claim unwind     | opus   | sonnet    | a failed stand-up leaves a release marker; the next claim acquires at once   |
| 4 resume no create | opus   | sonnet    | a resumed start never calls worktree.create; the lease is renewed, not freed |

## Acceptance Criteria

- [x] `parseMarker` rejects a body whose subject kind is not a frit
      marker kind, so a plan-authoring commit is never a lease marker
- [x] Every genuine marker — claim, beat, release, takeover, legacy
      decorated claim — still parses
- [x] `frit claim` on a merged, human-authored `plan/<id>` branch
      refuses as a lost race: no `scavenged:` line, no "set plan <id> to
      ✅"
- [x] The marker-kind list lives in one helper shared by `parseMarker`
      and `terminalMarkerKind`
- [x] A worktree on the plan's branch at a path other than the recorded
      lane path is reported by `frit orphans`
- [x] `frit reap --go` tears that foreign checkout down
- [x] A `frit claim` whose worktree stand-up fails releases the lease it
      minted and reports the refusal; the next `claim` acquires at once
- [x] `claim` and `start` unwind a failed stand-up through one shared
      helper
- [x] A `frit start` resumed from inside its own lane never calls
      `worktree create` and drives the agent in the current pane
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
