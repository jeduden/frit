---
id: 2608211326
title: The lease protocol completed through every verb
status: "🔳"
summary: >-
  Plan 2608202144 landed the lease atom, observation and takeover.
  This plan finishes the protocol: yield and rescue refs, the herdr
  veto with session binding and self-resume by token, parameters in
  .frit.yml with takeover backoff and the legacy-hold transition, the
  verb-state table wired through every verb, and the skills and docs
  rewritten to shipped behavior.
model: sonnet
depends-on: [2608202144]
phases:
  - n: 1
    title: yield and rescue refs
    status: "✅"
  - n: 2
    title: herdr veto, session binding, self-resume
    status: "🔲"
  - n: 3
    title: parameters and the legacy-hold transition
    status: "🔲"
  - n: 4
    title: the verb-state table through every verb
    status: "🔲"
  - n: 5
    title: skills and docs rewritten to shipped behavior
    status: "🔲"
---
# The lease protocol completed through every verb

## Goal

Finish the lease protocol so no enumerated scenario needs a human.
Fenced lanes exit clean. Live holders are never taken over while
their session answers. A restarted fleet of one resumes its own
lease at once. The knobs travel with each repository. Every verb
answers the contract table, and the instructions describe only what
ships.

## Context

The design record is
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md);
citations such as F9 and A2 point into it. The atom, the staleness
window, takeover and scavenge landed under plan 2608202144. What
remains is the machinery around a live fleet — sessions, parameters,
the long tail of verbs — and the rewrite of the instructions.

## Verb behavior, by state

The contract every verb implements, table-driven in its tests.
States of a plan's ref: absent, held-live (bound session or fresh
tip), held-stale (window matured), held-own (the lane's token
matches), released, landed-evidence.

| Verb       | absent        | held-live       | held-stale       | held-own        | released      | landed           |
| ---------- | ------------- | --------------- | ---------------- | --------------- | ------------- | ---------------- |
| `claim`    | acquire       | refuse, name    | take over        | resume          | re-acquire    | scavenge, refuse |
| `start`    | claim+lane    | refuse, name    | take over+lane   | resume lane     | re-acquire    | scavenge, refuse |
| `release`  | no-op, say    | refuse foreign  | refuse, say wait | release marker  | no-op, say    | scavenge         |
| `yield`    | park+teardown | park+teardown   | park+teardown    | refuse: holder  | park+teardown | park+teardown    |
| `pick`     | rank          | hide            | rank as takeover | rank as resume  | rank          | hide, flag       |
| `ready`    | list          | hide            | list, marked     | list, marked    | list          | hide, flag       |
| `board`    | show free     | show holder+age | show stale+age   | show own        | show released | show landing     |
| `orphans`  | —             | check lane      | report stale     | report lane gap | report        | report ref       |
| work verbs | refuse        | CAS or fence    | CAS or fence     | CAS renew       | refuse        | refuse           |

Every cell is a test: a scripted runner for what origin answers, an
explicit `now`, a state-file fixture, and the asserted output shape
in both renderings. No sleeps, no real network, no real clock.

## Tasks

1. Phase 1 — yield and rescue refs; `next` and `show` list them.
2. Phase 2 — herdr veto, session binding, self-resume by token.
3. Phase 3 — parameters in `.frit.yml`, takeover backoff, and the
   legacy-hold transition.
4. Phase 4 — the verb-state table wired through every verb.
5. Phase 5 — skills and [docs/claiming.md](../docs/claiming.md)
   rewritten to the shipped lease behavior.

## Phase 1: yield and rescue refs

A fenced lane exits clean instead of lingering. `frit yield` parks
local divergence to the rescue ref, tears the lane down through
herdr, and exits zero. The holder of a live lease is refused: yield
is for the fenced, not an alias for release (F4, F5).

RED:

- Yield in a fenced lane pushes the divergence to the rescue ref,
  create-only, asks herdr to tear the lane down, and reports what it
  parked.
- Yield in the lane that still holds the lease is refused.
- `next` and `show` list a plan's rescue refs, so stranded commits
  are found again.

GREEN: a `yield` verb in [cmd/frit](../cmd/frit/main.go). It reads
the lease token, reuses the rescue push, and hands teardown to
herdr. Rescue listing joins the `next` and `show` documents in
[internal/report](../internal/report/discovery.go).

Gate: the three RED cases pass in both renderings; no verb but yield
tears a lane down.

## Phase 2: herdr veto, session binding, self-resume

Liveness precedence: a positively live bound session vetoes takeover
and renews on the holder's behalf; no answer is no veto (F3, F10,
S61). Self-resume by token: a lane whose persisted token matches
origin's tip, with no live session on it, resumes with no window —
a fleet of one recovers as soon as it restarts (F9, F11, S3).

RED, with the fake-herdr idiom the who tests use:

- `claim` of a stale-held plan whose bound session herdr reports
  live is refused, and a beat is pushed on the holder's behalf.
- herdr unreachable, or the session unknown: no veto; the takeover
  proceeds.
- A lane whose persisted token equals origin's tip and whose session
  is gone resumes immediately: same epoch, no window consulted.
- The token is persisted in the lane's git dir on acquire and every
  renewal, so it survives the process.

GREEN: the session trailer is wired from `start`'s pane. The token
file lives beside the lane's git state. `claim`'s takeover path
reads the veto through [internal/herdr](../internal/herdr), and the
resume transition lands in
[internal/claim](../internal/claim/lease.go).

Gate: the four RED cases pass; a live holder is never taken over
while its session answers; no cross-machine clock appears.

## Phase 3: parameters and the legacy-hold transition

The knobs travel with the repository, and the old decorated holds
age out without a flag day. T, S_max and the backoff factor k live
in `.frit.yml` with the current defaults (F12). A takeover waits k·T
where k counts the takeover markers already in the chain, so
oscillation between two quiet agents damps out (F3).

RED:

- A repo declaring `takeover-window` and `sample-gap` sees staleness
  mature on its own clock; an undeclared repo keeps the defaults.
- A ref carrying one takeover marker matures at 2T, two at 3T.
- A legacy decorated hold still reads as a hold, and `orphans`
  reports it as a migration candidate naming the id-only ref.

GREEN: the parameters parse in
[internal/repocfg](../internal/repocfg/config.go) and thread into
the staleness fold. The backoff is read off the marker chain. The
migration row lands in `orphans`.

Gate: the three RED cases pass; a wrong `.frit.yml` value is a loud
parse error, not a silent default.

## Phase 4: the verb-state table through every verb

The contract table becomes the test suite. Every verb meets every
state and answers what its cell says, in both renderings. The gaps
close: a `release` verb for the holder, `board` showing stale holds
with their age, `orphans` reporting per the table.

RED: one table-driven test per verb over scripted origins, an
explicit `now`, a state-file fixture, and the asserted output shape
in table and JSON. No sleeps, no real network.

GREEN: the missing cells. `release` lands in
[cmd/frit](../cmd/frit/main.go), the stale column in
[internal/report](../internal/report/board.go), the orphans rows
beside it. Each is a small wiring of transitions that already exist.

Gate: every cell of the table is a passing test; goldens re-recorded
and the diff read.

## Phase 5: skills and docs rewritten to shipped behavior

The instructions catch up with the machine. The bundled skills and
[docs/claiming.md](../docs/claiming.md) describe the lease — the
id-only ref, takeover, yield, rescue refs — and nothing that no
longer exists.

RED: a docs sweep listing every claim the current text makes that
the landed phases falsified; each finding becomes an edit.

GREEN: rewrite the skill assets in
[internal/skills/assets](../internal/skills/assets) and
[docs/claiming.md](../docs/claiming.md). Re-run `frit skills` so the
dogfooded copies match the bundle.

Gate: `mdsmith check .` clean; no skill or doc names a behavior the
verbs do not have.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                                    |
| ------------------ | ------ | --------- | ------------------------------------------------------------------- |
| 1 yield, rescue    | opus   | sonnet    | yield parks and exits clean; next and show list rescue refs         |
| 2 veto, resume     | opus   | sonnet    | live session vetoes takeover; matching token resumes with no window |
| 3 parameters       | opus   | sonnet    | knobs travel with the repo; k·T backoff; legacy holds still read    |
| 4 verb-state table | opus   | sonnet    | every cell of the table is a passing test in both renderings        |
| 5 docs and skills  | sonnet | sonnet    | nothing described that the verbs do not do; mdsmith clean           |

## Acceptance Criteria

- [x] A fenced lane yields: divergence parked, lane torn down, exit
      zero; `next` and `show` list the rescue ref.
- [ ] A stale-held plan with a live bound session is not taken over,
      and the veto renews on the holder's behalf.
- [ ] A restarted lane whose token matches origin resumes its lease
      with no window.
- [ ] T, S_max and k are read per repository; takeover waits k·T.
- [ ] Every verb-state cell of the table is a passing test in both
      renderings.
- [ ] Skills and [docs/claiming.md](../docs/claiming.md) describe
      only shipped behavior; the manual-delete recovery and
      slug-branch prose are gone.
- [ ] A fenced session's next verb offers yield.
- [ ] Every mechanism in the research note's scenario matrix traces
      to at least one test named for it.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
