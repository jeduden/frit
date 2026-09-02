---
n: 1
title: Scavenge's post-delete confirmation stops reading unreadable as gone
status: "✅"
result: true
summary: Scavenge's delete-confirmation read switches from remoteHolder to remoteHolderErr; a read fault now returns a typed UnconfirmedDeleteError wrapping both the delete's and the confirmation read's faults and leaves the local ref untouched, instead of silently deleting it.
---
## Handoff

`Scavenge`'s delete-confirmation site in
[internal/claim/lease.go](../../internal/claim/lease.go) called
`remoteHolder(repoDir, opts.Remote, ref, run) != ""` to decide whether
a failed delete push actually landed — the same fold-to-absent
contract plan 2609012210 is fixing in `park`. An unreadable remote
read as `""`, so the check was false and Scavenge deleted the local
ref and reported success for a delete it never confirmed. The site now
calls `remoteHolderErr`: a read fault returns a new
`UnconfirmedDeleteError` (`PlanID`, `Ref`, `Err` wrapping both the
push's and the read's faults via a double-`%w` `fmt.Errorf`, so
`errors.Is` reaches either one through `Unwrap`) and the local-ref
delete below it is skipped. A confirmed-present ref keeps today's
wrapped delete error unchanged; a confirmed-gone ref keeps today's
local-ref cleanup unchanged.

**Proven.** Two new scripted-runner tests in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go),
`TestScavengeReportsAnUnconfirmedDeleteWhenTheConfirmationReadFails`
and
`TestScavengeKeepsTodaysDeleteErrorWhenTheConfirmationReadConfirmsStillPresent`,
follow the pattern already proven for `Scavenge`'s top-of-function read
(`TestScavengeErrsWhenTheRemoteCannotBeRead`) and for `casPush`'s own
`UnconfirmedPushError` in
[internal/claim/caspush_test.go](../../internal/claim/caspush_test.go):
a real repository via `originAndClone`/`Acquire`, wrapping `gitwt.Exec`
to force the delete push to fail and to script the second `ls-remote`
call's answer, counted separately from the first (the existence check
at the top of `Scavenge`). All nine pre-existing `Scavenge` tests stay
green unchanged. `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.

**A third call site this plan's scope never named.** `/code-review
high` on this phase's diff found that `park`'s own rescue-push
confirmation in [internal/claim/lease.go](../../internal/claim/lease.go)
(`remoteHolder(repoDir, opts.Remote, rescue, run) == tip`) still folds
a failed confirmation read to absent, the same class of bug this
plan's `Scavenge` fix corrects — and contradicts this plan's own
`plan.md` text, which asserts `park` already uses `remoteHolderErr`
(true only on the still-unlanded `plan/2609012210` branch, not on
`main`/this lane). Left unfixed here: it needs its own red/green
cycle outside this phase's touched functions, and is a call site
neither this plan's two phases nor 2609012210 name. Phase 2 or a
follow-up plan should weigh folding it in.

**A tree gap this phase's landing closed first.** This lane's claim on
plan 2609021114 was minted before the plan's own definition had landed
anywhere this branch's history could see: the plan was authored on the
sibling `plan/2609012210` branch (`edc72fa`), which itself is not yet
merged to `main`. The plan-definition commit was cherry-picked into
this lane's history (amended to also carry the `PLAN.md` catalog row
the mdsmith merge driver dropped during the cherry-pick, and to
de-link two `plan.md`/`phase-1.md` references to
`internal/claim/park_test.go`, which does not exist on this branch
since plan 2609012210's own execution is still unlanded) so
`mdsmith check .` and `frit phase` could resolve it. `remoteHolderErr`
itself predates 2609012210 and needed no changes; nothing in this
phase depended on park's own fix landing first.

**What Phase 2 inherits.** `UnconfirmedDeleteError` is Scavenge-local
and not reused verbatim by Phase 2 — `Yield`'s still-held check needs
its own typed error (the plan's `UnconfirmedYieldError`) since its
wording is about a refusal before `park`, not a delete. The same
scripted-runner shape (a real repo, `gitwt.Exec` wrapped to force one
call to fail and script a later read) applies directly: `Yield`'s
still-held `ls-remote` is its *only* remote read, so Phase 2's script
needs no call-counting the way this phase's did.
