---
id: 2608221754
title: The supervisor acts on a lane it does not stand in
status: "🔳"
summary: >-
  frit's deserted-hold recovery assumes the actor re-enters the lane's
  own worktree: resume, release and the yield way-out are all
  inOwnLane-gated. A supervisor orchestrating lanes from the primary
  clone hits dead ends when a session is gone but its checkout
  survives — the lane is invisible to orphans, and start/claim/yield
  give a bare refusal or a destructive takeover. Map the whole
  supervisor surface, then close it where orphans hides a
  herdr-confirmed-dead lane behind its own dead worktree's
  still-current token.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: orphans surfaces a dead lane its own token hides
    status: "✅"
  - n: 2
    title: the S77 park-first guard fires from outside the lane
    status: "🔲"
---
# The supervisor acts on a lane it does not stand in

## Goal

A supervisor sees and acts on a deserted lane it does not stand in,
without cd-ing into the worktree or running raw git. A session gone
with a surviving checkout is surfaced and has a supervisor way out.

## Context

A supervisor is a session that orchestrates lanes from the primary
clone — a monitor-and-merge loop is the case that surfaced this. It
never stands in a lane's own worktree, and it names plans by explicit
id. Two structural facts govern its whole action surface, and a full
map of the verbs was taken before this plan.

The master gate is `inOwnLane` in
[cmd/frit/claim.go](../cmd/frit/claim.go). It resolves cwd to a plan
id and demands it equal the target. So every own-lane fast path is
unreachable from the primary: `resumeOwnLease`/`startResumeTip`
(resume), `desertedRefusal` (the yield hint), `ownToken` (release)
and `tearDownLane`. The `guardForeign` preflight never fires either,
because it only runs on an empty selector; an explicit id passes it.
So a supervisor is not refused as foreign, but it also gets no
recognition as a supervisor — it is just some clone that is not this
lane.

The deserted-hold recovery in plan 2608212346 is the safety net this
plan extends, and it is deliberately lane-local. Its way-out —
`start` resumes in place or names `yield` — is gated on `inOwnLane`,
so a supervisor never sees it. This plan does not rewrite that
recovery; it adds the supervisor's view and route beside it, reading
the same herdr veto the deserted kind already reads.

The map found six gaps where a supervisor has no frit-native action
without entering the lane or running raw git:

1. A held lane whose session is gone but herdr cannot confirm it dead
   (`!Stale && !Dead`), checkout surviving: `start`/`claim` give a
   bare `already held`; `orphans` names nothing. This is the case the
   loop hit, and the hardest — frit cannot tell it from a quiet live
   lane without a better liveness read or a supervisor override.
2. A herdr-confirmed-dead hold whose surviving checkout still carries
   a token matching origin's tip: `desertedHeld` drops it via
   `resumableFromAnyLane` in [cmd/frit/main.go](../cmd/frit/main.go),
   so `orphans` hides it, though the only lane that could resume it is
   itself the dead one.
3. Takeover of a dead or stale hold with commits past its token
   orphans that work: `mintOrTakeOver` in `claim.go` runs the
   takeover with the S77 park-first guard disabled outside the lane.
4. Teardown of a deserted lane's surviving checkout is unreachable
   from the primary: `tearDownLane` in
   [cmd/frit/yield.go](../cmd/frit/yield.go) removes only the caller's
   own worktree, and `reap` in [cmd/frit/reap.go](../cmd/frit/reap.go)
   never classifies a held-and-checked-out lane as an orphan
   ([internal/lanes/lanes.go](../internal/lanes/lanes.go)).
5. `release` is lane-local for any held plan: `releaseHeld` needs
   `ownToken`, so a supervisor is always bounced to `claim`.
6. An unstaffed hold that looks live (`!Stale && !Dead`): `reap`
   refuses until the window matures or herdr confirms death.

Gap 2 is the cleanest proving slice. herdr already confirms the
session dead, so the fix is one predicate. It is testable against an
in-memory fleet with the fake-herdr idiom the `who` tests use. It
makes the deserted lane visible to a supervisor, which every later
route depends on. The reuse seam is
`desertedHeld`/`resumableFromAnyLane` in `main.go` — the same fold
`board` and the deserted kind read. The protocol contract is
[lease-protocol.md](../docs/research/lease-protocol.md), S76 and S77.

## Tasks

1. `orphans` surfaces a herdr-confirmed-dead hold whose only resumable
   checkout is its own dead lane's, instead of hiding it.
2. The supervisor way out for a surfaced deserted lane from the
   primary — resume in place when the checkout is clean, else park the
   suffix (S77) first — realized not as a new verb but as the S77
   park-first guard firing from outside the lane, so the takeover
   `claim`/`start` already run becomes safe to use as the resume.
3. (determined after Phase 2) Teardown of a deserted lane's surviving
   checkout from the primary, and the harder `!Stale && !Dead` cell —
   an acknowledged supervisor override or a tightened liveness read.

## Phase 1: orphans surfaces a dead lane its own token hides

`desertedHeld` excludes a hold when `resumableFromAnyLane` is true.
That predicate counts a worktree whose token matches origin's tip as a
lane that could self-resume — but when that worktree's own bound
session is the dead one, no live lane can resume it, and the hold is a
silent dead end. A dead session's own checkout must not hide its hold.

RED, against an in-memory fleet with the fake-herdr idiom the `who`
tests use, in [cmd/frit](../cmd/frit):

- A held plan, herdr confirms its bound session gone, window not
  matured, a surviving checkout whose token matches origin's tip:
  `orphans` lists it as deserted. Today it lists nothing.
- The same plan with a live bound session: not listed — a live lane
  can resume it, so the exclusion still holds.
- The same plan whose surviving checkout is behind origin's tip: still
  listed as deserted, exactly as plan 2608212346 already lists it — no
  regression.
- A matured hold stays a stale-held takeover candidate, not a deserted
  hold: the two kinds do not collide.

GREEN: gate the `resumableFromAnyLane` exclusion in
[cmd/frit/main.go](../cmd/frit/main.go) on the resuming worktree's
session being live. A dead session's own resumable token then no
longer suppresses the deserted kind. The table prints it with the
deserted label already in
[internal/report/orphans.go](../internal/report/orphans.go), and
`--json` carries it. Re-record the report golden files in
[internal/report/testdata](../internal/report/testdata) and read the
diff.

Gate: the four RED cases pass; the deserted and stale-held kinds do
not collide; the JSON key is always present as `[]`; `go test ./...`
and `mdsmith check .` are clean.

## Phase 2: the S77 park-first guard fires from outside the lane

The supervisor's way out is the verbs frit already has, not a new
one. For a herdr-confirmed-dead, unmatured hold, `claim` and `start`
already run the right transition from the primary. `mintOrTakeOver`
in [cmd/frit/claim.go](../cmd/frit/claim.go) seizes it at epoch E+1,
live-session veto intact. `yield <id>` already parks the branch's
local suffix from any clone, because `localRef` and `claim.Yield`
read shared git state, never cwd. The missing piece is the S77
guard. `desertedRefusal` is `inOwnLane`-gated, so a takeover from
the primary silently orphans whatever the dead lane committed past
its token (gap 3). Phase 2 adds the guard's cwd-free sibling in
front of the takeover. It refuses when the plan branch's local tip
diverges from the hold tip, and names `frit yield <id>` as the park.
Clean means the local `plan/<id>` ref is absent, equal to, or an
ancestor of the tip the takeover would CAS on; the ref is shared by
every worktree, so the primary reads it. Uncommitted working-tree
dirt stays out of scope, exactly as it does for `yield` itself. The
takeover that then proceeds is the resume in place: same branch,
epoch E+1, herdr standing the pane back up at the conventional lane
path.

RED, against the real-repo-plus-fake-herdr fixtures
`deserted_test.go` already builds, in [cmd/frit](../cmd/frit):

- A held plan, herdr confirms its bound session gone, window not
  matured, one local commit on the plan branch past the hold tip:
  `claim <id>` run outside the lane refuses and names
  `frit yield <id>`. Today it takes over and orphans the commit.
- The same fixture through `start <id> --go`: the same refusal — the
  two verbs share the guard, as they share `desertedRefusal`.
- The same hold with the plan branch clean at the hold tip:
  `claim <id>` from the primary takes it over at epoch E+1 — the
  resume-in-place path, pinned against regression.
- The same clean hold whose bound session herdr answers live: the
  veto still refuses and names the holder — the guard adds no route
  past `mintOrTakeOver`'s live-session check.
- From inside the lane, the existing wording still wins:
  `TestStartNamesYieldForADesertedLaneOnThisHost` passes unchanged.

GREEN: a `parkFirstRefusal` beside `desertedRefusal`, called from
claim's `Run` and `buildStart` right after the own-lane refusal. It
is skipped when the gather withheld the coordinate; the
ambiguous-repo refusal already stands there. For a held, dead,
unmatured plan it reads `localRef`
([cmd/frit/yield.go](../cmd/frit/yield.go)). Then it asks
`git -C <dir> merge-base --is-ancestor` — plumbing, exit code only —
whether that tip is reachable from `plan.HoldTip`. Divergence
refuses. Nothing in [internal/claim](../internal/claim) changes:
takeover, veto and park are composed, not extended. No new verb
means no new skill. The plan-tidy asset in
[internal/skills/assets](../internal/skills/assets) does gain the
supervisor route, within its token budget: orphans' deserted row →
`yield <id>` from the primary, then `claim <id>`. The dogfood copies
are regenerated so `TestDogfoodCopiesMatchCanonical` stays green.

Deferred to Phase 3 with task 3: teardown of the surviving checkout
from the primary. A surviving lane at a non-default path, honored
when the pane is stood back up, is deferred with it.

Gate: the five RED cases pass; the live-session veto is pinned by
its own case; `go test ./...` and `mdsmith check .` are clean.

## Execution

Tier is per phase, set by the most demanding ingredient. Phase 1
implements from written assertions against a settled seam, so it is a
sonnet slice; the design of the supervisor route stayed opus and is
now recorded in Phase 2 itself. Phase 2 composes existing primitives
behind one new refusal, so its implementation is a sonnet slice for
the same reason Phase 1's was.

| Phase                 | Design | Implement | Gate that catches a wrong answer                                  |
| --------------------- | ------ | --------- | ----------------------------------------------------------------- |
| 1 orphans surfaces it | opus   | sonnet    | four RED cases pass; deserted and stale-held kinds do not collide |
| 2 park-first guard    | opus   | sonnet    | an unparked suffix refuses takeover from outside; the veto stands |

## Acceptance Criteria

- [x] `frit orphans` lists a herdr-confirmed-dead hold whose only
      resumable checkout is its own dead lane's
- [x] A live bound session keeps that hold off the deserted list
- [x] A behind-the-tip deserted lane still lists, with no regression
- [x] A matured hold stays a stale-held candidate, not a deserted hold
- [x] The `--json` deserted key is always present as `[]`
- [ ] `frit claim <id>` run outside the lane refuses a dead,
      unmatured hold whose branch carries an unparked suffix, naming
      `frit yield <id>`
- [ ] `frit start <id>` shares that refusal through the same guard
- [ ] A clean deserted hold is taken over from the primary at epoch
      E+1 — resume in place
- [ ] A live bound session still vetoes that takeover
- [ ] plan-tidy names the supervisor route; dogfood copies match the
      canonical assets
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
