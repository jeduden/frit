---
n: 1
title: Refuse before the mint when herdr is not present
status: "✅"
result: true
summary: >-
  claim, start --go and pick --go refuse locally on a host with no
  herdr installed, naming it and pushing nothing; an unreachable but
  installed herdr keeps the old unwind. C15 and C16 cover it.
---
# Phase 1 result

## Handoff

A host with no herdr on `$PATH` now learns so before anything reaches
origin. `claim`, `start --go` and `pick --go` answer `refused: plan
<id> herdr not found; nothing claimed`, carry the raw exec error as a
herdr problem, and push no claim or release marker. The gate runs
last, so a held, blocked or done plan keeps its own reason, and a
dry-run `start` still composes.

Only a missing executable is refused. An installed herdr whose socket
will not answer still mints and unwinds, as S60 and S61 pin. The
`plan-pick` skill now says what the refusal means: nothing reached
origin, so there is nothing to release.

**Verified by.** Unit tests for the probe and for each verb, written
red first. C15 and C16 run the built binary under a `$PATH` holding
git alone. With the claim gate disabled, C15 failed on exactly the
issue's `worktree not stood up … executable file not found` output.

**Fixtures that leaned on an absent herdr.** C12, S95 and S99, and
two claim unit tests, drove `claim` or `start --go` with no fake herdr
at all. They passed only because the build box has none, and their
race was lost before herdr was needed. The gate now refuses first
there, so each names `herdr is installed on this host`. The cost, by
design: on a headless host, a claim that would have lost its race to
a landed winner reports herdr not found instead of already landed,
and leaves the landed ref for a host with herdr to scavenge.

**For phase 2.** Nothing here decides the herdr-less lane. The probe
it adds is the natural switch for it: a lane with no worktree would
skip this gate rather than remove it.
