---
n: 1
title: The five lease-API rows of process death run for real
status: "✅"
result: true
summary: >-
  S1, S2, S7, S10 and S11 drop `@pending` and run as real scenarios in
  the new `cmd/frit/bdd_process_death_test.go`: a claim killed before
  any local write leaves origin untouched and a retry acquires clean
  at epoch 1; one killed after the local write but before the push is
  refused on its own retry as a local-diverges branch while a second
  machine claims fresh; `resetWindow` restarts an observer's matured
  window fresh, one sample, on the tip a takeover actually moved to;
  a takeover started after the holder pushed a phase commit is a
  child of that pushed tip and carries the work; one started after
  the holder only committed locally is a child of the last tip that
  reached origin, and the local-only commit is reachable from nowhere
  on origin. The file registers itself the way `bdd_lease_test.go`
  does, reuses "holds the lease for plan", "commits work on the lane
  it never pushes" and "takes the lease over" as-is, and keeps its own
  state in a `deathState` reached through `section[T]` rather than a
  field on `world`.
---
## Handoff

**Rows needing a step the lease world lacked.** All five did, since
`bdd_lease_test.go`'s vocabulary only covers a claim that already
succeeded. S1 needed a Given for a plan nobody has touched yet
("has a claimable plan") and a Then that performs the retry itself
("retries and acquires the lease at epoch 1"), following the pattern
S16 already sets of a Then step that also acts. S2 needed a Given
that mints the local hold-branch commit a real claim writes without
ever pushing it, and a fresh claim step for the second machine. S7
needed a way to seed a matured window (the existing `seedWindow` test
helper from `claim_test.go`, reused directly — same package, no new
code) and a step driving `resetWindow` itself at a fixed instant. S10
needed a step that pushes a real work commit to origin, and — this is
the one genuinely new lease-level step — a takeover that CASes from
origin's *current* tip rather than the tip the world's own earlier
acquire recorded, since `bdd_lease_test.go`'s "takes the lease over"
hard-codes the stale acquire tip and would lose its CAS against a
tip a push had since moved. S11 reused "takes the lease over" as-is,
since nothing pushed after the acquire means the stale tip and the
current one are the same value.

**A finding, not a fix.** S2 as first drafted had the wrong event
order: the row's own words ("another machine claims... the first
machine's stale local ref is refused on its retry") read as the
retry coming after the other machine's win, but `claim.Acquire`'s
local-diverges guard (`refuseDivergingLocalBranch`) only runs on the
fast path where origin's ref is still absent — once a second machine
has already claimed and pushed, the *same* stale local branch instead
loses to a plain `HeldError`, not `LocalDivergesError`. The correct
order is the first machine's own retry running first (finding its
local branch diverges, refusing before it ever touches origin, so
origin stays untouched) and only then the second machine claiming
fresh into the ref its rival's refused retry left bare. The scenario
now runs in that order; no assertion was weakened to make it pass.

**The gate command in this plan's own phase-1.md does not match.**
`go test ./cmd/frit -run 'TestFeatures/S(1|2|7|10|11)_'` selects
nothing: `TestFeatures` names each subtest `"<id>: <title>"`
(`bdd_test.go`), so the actual name is `S1:_killed_before...`, not
`S1_killed...`, and `-run`'s unanchored regex never finds a bare `_`
straight after the digits. `TestFeatures/S(1|2|7|10|11):` is the
working form — the same fix the harness's own original handoff
(`plan/2609012000`) already used (`'TestFeatures/^S16:'`) before this
family of gate strings was copied into the seven sibling plans with
the colon dropped. All seven carry the same stale pattern; worth a
one-line fix across them, out of scope here since it touches no file
this plan owns.

**What S3..S6 will need.** None of the four is reachable over the raw
lease API alone the way these five were:

- S3 (killed mid-push, server committed) and S4 (self-resume) are the
  resume path — `claim.Resume` / `RenewToBind` — not `Acquire` or
  `Takeover` directly; likely exercised through the `resume`/`claim`
  verb in `cmd/frit` rather than the lease package's exported funcs
  alone.
- S5 (board shows a held lane with no session) needs `board`'s
  rendering plus a hold with no session bound — the herdr fake
  (`startHerdr`, `withHerdr`) wired in for the first time in this
  section, reporting no live session for the lane.
- S6 (an idle agent never renews; the veto cannot fire with nothing
  bound) needs the same herdr fake so `herdr.SessionLive` has a
  concrete false to return, exercising `mintOrTakeOver`'s veto-absent
  branch.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S(1|2|7|10|11):'`
reports five PASS, none SKIP; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean;
`go test ./internal/scenario` (the bijection gate) stays green.
