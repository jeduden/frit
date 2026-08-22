---
id: 2608212346
title: A deserted hold is seen and has a way out
status: "✅"
summary: >-
  A held lane whose bound session herdr can no longer see, and whose
  token cannot self-resume because it is absent or behind origin's
  tip, is a silent dead end: pick, start, nudge and open all refuse
  it, and orphans stays quiet until the takeover window matures.
  Surface it as its own orphan kind, read from the herdr veto rather
  than the staleness window, then give one verb a way out — resume
  the pane in place when the tip allows, or name the yield that
  retires it (S76, S77).
model: sonnet
depends-on: []
phases:
  - n: 1
    title: orphans names the deserted hold
    status: "✅"
  - n: 2
    title: a verb resumes or retires it
    status: "✅"
---
# A deserted hold is seen and has a way out

## Goal

A held lane can lose its pane while its work ref still reads as a
hold. When self-resume cannot recover it, `frit orphans` surfaces it,
and one verb resumes or retires it. A dead pane is never a silent
dead end.

## Context

The gap was hit twice. A lane's pane closed, its work ref still read
as a hold, and every resume verb refused. `pick` and `ready` skip it:
[ready.go](../internal/discovery/ready.go) offers only matured-stale
held plans as candidates. `start`'s `startResumeTip` in
[start.go](../cmd/frit/start.go) returns `""` when the persisted token
is absent or behind origin's tip, so the `already held` refusal fires.
`nudge` and `open` refuse with `no live lane`.

`orphans` already carries kinds — unstaffed, stranded, empty, prunable,
migratable, stale-held — in [orphans.go](../internal/report/orphans.go),
assembled in [main.go](../cmd/frit/main.go). Its `staleHeld` filter
requires `p.Stale`, so a freshly deserted lane is invisible until the
takeover window T matures. That is the reuse seam: a new kind beside
the others, read from the same observation fold `board` and `claim`
use, gated on the herdr veto rather than the window.

Self-resume already exists — `resumeToken` and `claim.Resume` in
[start.go](../cmd/frit/start.go) and [lease.go](../internal/claim/lease.go)
— and it is the right recovery when the tip allows it. This plan does
not rewrite it; it routes a deserted lane into it, and names the yield
path when the token is behind the tip. herdr liveness is read through
[observe.go](../internal/observe/observe.go), tested with the
fake-herdr idiom the `who` tests use. The protocol contract is
[lease-protocol.md](../docs/research/lease-protocol.md); S76 and S77
were added there for this work.

## Tasks

1. Surface the deserted hold as an `orphans` kind, from the veto.
2. `start` names `yield` for a deserted hold read from its own lane,
   ahead of the ordinary readiness refusal, so a bare takeover never
   orphans what that lane committed past its persisted token.

## Phase 1: orphans names the deserted hold

A deserted hold is a held plan whose bound session herdr reports gone,
that self-resume cannot recover — token absent or behind origin's tip
— surfaced regardless of the staleness window. `orphans` names it as
its own kind, distinct from a matured stale-held takeover candidate.

RED, against an in-memory fleet with the fake-herdr idiom the `who`
tests use:

- A held plan, herdr reports its bound session gone, token behind the
  tip, window not matured: `orphans` lists it as deserted.
- The same plan with a live bound session: not listed — the veto
  holds.
- The same plan whose token matches origin's tip: not listed —
  self-resume can recover it, so it is not a dead end.
- A matured stale-held plan stays a stale-hold, not a deserted hold:
  the two kinds do not collide.

GREEN: a `Deserted` kind on `OrphanRepo` in
[orphans.go](../internal/report/orphans.go), populated in
[main.go](../cmd/frit/main.go) from the observation fold, gated on the
herdr veto and the resume-token check. The table prints it with its
own label; `--json` carries it with the others. The report golden
files in [testdata](../internal/report/testdata) are re-recorded and
the diff read.

Gate: the four RED cases pass; the JSON key is always present as `[]`;
`go test ./...` and `mdsmith check .` are clean.

## Phase 2: a verb resumes or retires it

A deserted hold read by Phase 1 gets one verb that acts on it. When
the lane is on this host and its token can self-resume, the verb
rebuilds the pane in place. When the token is behind origin's tip, the
verb names the `yield` that retires the lane, parking any suffix to a
rescue ref rather than leaving a dead end (S77).

RED, against the same fake-herdr idiom: run from a deserted lane's own
worktree whose token is behind origin's tip, `start` refuses and names
`yield` instead of taking the lane over from itself
(`TestStartNamesYieldForADesertedLaneOnThisHost`). The same lane with
a resumable token is unaffected. `startResumeTip` already recovers it
ahead of the refusal.

GREEN: `desertedRefusal` in [start.go](../cmd/frit/start.go). It is
gated on `inOwnLane` in [claim.go](../cmd/frit/claim.go) — the
identity check `ownToken` already makes, reused rather than
reimplemented. `claim` shares the same `mintOrTakeOver` transition
`start` does, so it reads the same refusal before its own readiness
check. The gate is that a deserted lane on this host is resumed with
no manual branch surgery, and one whose tip cannot resume is pointed
at `yield`; the verb-state table gains no silent cell.

## Execution

Tier is per phase, set by the most demanding ingredient. The design
is settled in this plan and the protocol note, so both phases
implement from written assertions.

| Phase              | Design | Implement | Gate that catches a wrong answer                                  |
| ------------------ | ------ | --------- | ----------------------------------------------------------------- |
| 1 orphans names it | opus   | sonnet    | four RED cases pass; deserted and stale-held kinds do not collide |
| 2 resume or retire | opus   | sonnet    | a deserted lane resumes in place, or is pointed at yield          |

## Acceptance Criteria

- [x] `frit orphans` lists a deserted hold before the window matures
- [x] A live bound session or a resumable token keeps it off the list
- [x] One verb resumes a deserted lane in place, no branch surgery
- [x] A behind-the-tip lane is pointed at `yield`, not left dead
- [x] S76 and S77 read true against the shipped behavior
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
