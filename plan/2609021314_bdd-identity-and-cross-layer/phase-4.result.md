---
n: 4
title: "A reset window, a fenced release and a doc boundary: S62, S63, S65, S66 run real"
status: "✅"
result: true
summary: >-
  S62, S63, S65 and S66 drop `@pending` and run as real scenarios: a
  stale hold's own holder pushing while herdr is unreachable resets
  the observation window, so a claimant over it waits again rather
  than taking over; a lane whose session herdr swears is live is
  still fenced out once a foreign takeover has moved the ref, proving
  liveness only ever vetoes an incoming takeover and never rescues a
  lane a CAS has already lost; a herdr that answers but names nobody
  lets a stale hold's takeover complete cleanly, unlike an
  unreachable herdr's own failed stand-up; and a lane's marker and
  token, read back after the fact, carry no host anywhere — the
  documented boundary an NFS-shared clone runs into. All sixteen rows
  this plan has landed so far stay green together.
---
## Handoff

**All four rows landed exactly as phase-4.md predicted, with one
finding.** S62's pushed commit end-to-ends
`observerResetsTheWindowOnTheNewTip`'s own direct-call proof
(`bdd_process_death_test.go`) through a real claimant's refusal: a
worktree add on the stale holder's own clone, a raw commit, a push,
then `claimRefusal` reports the plan held but not yet stale, before
`mintOrTakeOver` is ever reached. S65 shows the other side of S61's
own coin: `herdrShowsNoAgentOnTheLane`'s reachable, empty handshake
lets the takeover's own worktree stand-up succeed, so the claim
completes at epoch 2 with a plain takeover marker, never a
release-wrapped unwind. S66 asserts the doc boundary the plan's own
Context section named: `leaseMessage` writes `lane:` as `opts.Lane`
verbatim, and `claim.TokenPath` resolves entirely under that same
path's git directory — no verb runs, and the two Then steps read the
fixture's own marker and token straight back.

**The finding: S63 is proof by absence.** `release`'s own fencing —
Phase 1's own S86 already proved it — never consults herdr at all. A
session herdr would swear is alive buys a fenced-out lane nothing,
not because some check weighs liveness against the CAS and the CAS
wins, but because no such check exists on `release`'s own path. The
scenario passes with `herdrConfirmsTheLanesOwnSessionIsLive` installed
exactly as it would without it. That is the row's whole point:
liveness only ever governs whether an *incoming* takeover is vetoed,
never whether a lane's own next verb succeeds after one has already
landed. Nothing here was weakened to reach green; the fake stays in
the scenario because the doc's own row names it, even though the code
path it exercises does not read it.

**No finding against the product otherwise.** Every assertion in all
four rows passed on the first run once each fixture was built; no
check was loosened.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|62|63|64|65|66|73|76|77|86):'`
reports sixteen PASS, none SKIP; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean; `go test
./internal/scenario` (the bijection gate) stays green.

**What S72 and S74 — this plan's last two rows — still need.** Both
turn on two machines racing the verbs, the shape no row in this file
has built yet. `cloneRepoIntoRoot`
([bdd_process_death_test.go](../../cmd/frit/bdd_process_death_test.go))
already clones a repo's origin into a second machine's own `--root`,
the fixture both rows want, but a genuine race needs more than a
second clone: S72 (claim and start race on one host) needs two verb
invocations actually contending on one push, not one run after
another, so the next phase's own fixture is the part this plan has
never needed before — two goroutines calling `run` concurrently
against the same origin, and reading which one's push landed off the
result rather than off timing. Neither goroutine may `t.Chdir`,
since both would race the same process-wide directory; each passes
`--root` instead, the way every step in this file already does. S74
(same plan id in two repos) is simpler: two `claimableRepo`s under
different repository names sharing one plan id, then a read-back that
lanes key on `host:repo:id`, not `host:id` alone, and that pane names
this file's own herdr fakes send carry the repo. Neither row needs a
new herdr shape; both are the file's last two rows to close before
the plan's Acceptance Criteria are met.
