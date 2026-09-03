---
n: 4
title: S32, two same-host start sessions racing, runs for real
status: "✅"
result: false
---
Convert S32 from `@pending` into a passing scenario. It is the one row
in this plan's whole matrix that exercises `frit start`, not `frit
claim` or the lease API alone. Every other row proves the lease's own
CAS, or a verb's window-and-veto logic. S32 proves the guard both of
those miss: `startLiveLaneRefusal` in
[cmd/frit/start.go](../../cmd/frit/start.go). It reads herdr's own
pane list directly, keyed on repository and branch — independent of
any marker's holder or bound session (issue #126).

**Assumes.** Traced through `buildStart`, `startRefusal` and
`liveLaneFor`:

- `startAcquire` mints and pushes the claim before `standUpLane` ever
  asks herdr to stand a worktree up — so by the time a second,
  same-host `start` call's own discovery snapshot runs, the plan is
  genuinely held.
- `startRefusal` calls the same `claimRefusal` `frit claim` uses.
  `claimRefusal`'s own first move is checking membership in
  `discovery.Ready(res.Plans)` — and a *matured* hold's own row is
  included there anyway (S76), a legitimate takeover candidate despite
  `p.Held` being true. Only past that gate does `buildStart` ever
  reach `startLiveLaneRefusal`. A fresh, unmatured claim refuses
  earlier, via the ordinary "already held" wording — never reaching
  the live-lane check at all.
- `startLiveLaneRefusal` (skipped outright for a resume) calls
  `liveLaneFor`, which reads herdr's pane list, resolves each pane's
  `cwd` to a worktree root and branch (`herdr.Resolve`,
  `rev-parse --show-toplevel` then `symbolic-ref --short HEAD`), and
  matches a pane whose branch is one of the plan's own hold patterns
  and whose worktree's repository name matches the plan's.
- Herdr's own `worktree create` RPC — the call `standUpLane` makes,
  carrying the exact `--cwd <path>` frit itself chose — never creates
  a real git worktree in production; frit hands the whole job to
  herdr and never runs `git worktree add` itself. A fake that only
  echoes a `pane_id` therefore leaves nothing on disk for a later
  `herdr.Resolve` to find.
- `startHerdr()` ([start_test.go](../../cmd/frit/start_test.go)) is
  the shape of a working escalation's fake — `worktree create`, `pane
  current`, `agent list`, everything else a silent no-op — but its
  `agent list` is a static, canned pair of panes carrying no `cwd` at
  all, so it can never reflect a worktree back. `recordingHerdr` and
  `liveLeaseFixture`
  ([start_test.go](../../cmd/frit/start_test.go)) already prove the
  shape a live pane's `cwd` must take to resolve — a real git checkout
  on the plan's branch — but only as a static fixture built before
  `start` ever runs, standing in for a different machine's pre-existing
  lane, not the one this plan's own first call creates.

**Value.** A lease is an atomic ref push; a worktree and a pane are
not. Two `frit start --go` calls on the same host, close enough
together, cannot both win the CAS, but the loser could still clobber
the winner's just-created worktree or double up a pane on the same
branch if nothing stood between the CAS and the stand-up. This is the
one guard proving that: a regression here reopens issue #126 with no
scenario to catch it.

**RED.** Drop `@pending` from S32 in
[races.feature](../../features/races.feature):

```gherkin
@S32
Scenario: two same-host sessions race
  Given "this host" is ready to start plan 7
  When "this host" starts plan 7
  Then the start succeeds, standing this host's own lane up
  When the hold's takeover window has matured
  And "this host" starts plan 7
  Then the second start is refused, naming the lane the first stood up
```

Run `go test ./cmd/frit -run TestFeatures -v`. The new subtest fails
on the undefined steps. That is the red — commit it.

**GREEN.** Extend `cmd/frit/bdd_host_death_and_races_test.go`:

- `^"this host" is ready to start plan (\d+)$` — `isolate`,
  `claimableRepo`, install a stateful herdr fake (below) via
  `withHerdr`, mark `cliState.herdrSet`.
- `^"([^"]+)" starts plan (\d+)$` — runs `frit start <id> --phase 3
  --go --root <dir>`, capturing stdout into `cliState.out`. After the
  run, reads origin's current tip (`claim.RemoteTip`) into `w.lease` —
  the CLI never sets it, and `theHoldsTakeoverWindowHasMatured`
  (reused unchanged) needs it to seed the window against the real tip
  a stale check would otherwise see as freshly moved.
- `^the start succeeds, standing this host's own lane up$` — asserts
  `cs.out` contains `"started plan"`.
- `^the second start is refused, naming the lane the first stood up$`
  — asserts `cs.out` contains `"refused"`, `"already sits on lane"`
  and the plan's own branch (`claim.Branch`).
- A stateful herdr fake, `liveLaneHerdr`: remembers nothing until it
  intercepts `worktree create`, at which point it parses the `--cwd`
  argument frit itself supplied, actually runs `git clone --branch
  <plan-branch> <origin-url> <that path>` for real — the one piece of
  work herdr would have done in production — and only then answers
  with a canned `pane_id`. Every `agent list` call from then on
  reflects a single live pane at that same path; before it, an empty
  roster. `pane current` echoes the same `pane_id`.

**Guard the edges.** Three traps:

- The maturing step between the two calls is load-bearing, not
  incidental: without it, the second call's own `claimRefusal` refuses
  first, with the ordinary "already held" wording, and
  `startLiveLaneRefusal` is never reached at all.
- The fake's real clone must target the *plan's* branch
  (`claim.Branch(planID)`, e.g. `plan/7`), not the repository's
  default branch — `herdr.Resolve`'s `symbolic-ref --short HEAD` reads
  whatever is checked out, and `liveLaneFor` matches on that name
  against the plan's own hold patterns.
- `w.lease.Tip` must be read back from origin after the *first* start
  call, not before it — the CLI-driven step never populates it any
  other way, and a stale or empty tip would seed the window against
  the wrong ref state.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S32:'` passes,
reported PASS, not SKIP. `go test ./internal/scenario` (the bijection
gate) stays green. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

This closes the plan: every row of S14..S19 and S26..S32 runs for
real, none `@pending`. Write the handoff to `phase-4.result.md`, flip
`plan.md`'s Acceptance Criteria and its own `status:` to ✅, and run
`mdsmith fix PLAN.md`.
