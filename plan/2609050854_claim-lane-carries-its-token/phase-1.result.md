---
n: 1
title: A claim-only lane persists its token once it stands
status: "✅"
result: true
summary: >-
  `claim` now writes its own lane's token the moment herdr reports the
  worktree stood up, through a newly exported `claim.WriteToken`
  shared with the unexported `persistToken` every other transition
  already called. `start`'s fresh-worktree branch calls the same
  write, so its token no longer depends on `bindSession`'s later
  renewal, which only fires once herdr's own session lookup answers
  non-empty. A claim-only lane now releases and resumes from inside
  itself exactly as a lane `start` created does, with no takeover
  window to wait out — proven against `liveLaneHerdr`, which runs a
  real `git worktree add`, in `cmd/frit/claim_test.go`,
  `cmd/frit/start_test.go` and `cmd/frit/release_test.go`, and end to
  end by the new `@S92` scenario in `features/lifecycle.feature`,
  mirrored as a row in `docs/research/lease-protocol.md`.
  A write that fails once the worktree already exists — as opposed to
  the routine, silent skip when nothing is on disk yet — surfaces as a
  warning on the claim document rather than a silent no-op, so a
  standing lane's misfortune can be found rather than trusted.
---
## Handoff

**Done.** `internal/claim.WriteToken(lane, planID, tip, run) error` is
the exported call `standUpClaimWorktree` in `cmd/frit/claim.go` and
the fresh-worktree branch of `laneStandUpPane` in `cmd/frit/start.go`
both make once `herdr.WorktreeCreate` returns. `persistToken` is now a
thin wrapper over it, unchanged in every other caller (`Renew`, `RenewToBind`,
`Takeover`, `pushClaimMarker`). `claim`'s own write reports a non-nil
error as `doc.Warn`; `start`'s is dropped, the same best-effort cost
`bindSession`'s own renewal already carries, since the lease is on the
remote either way and the checkout stood up regardless.

**Proven.** `TestClaimPersistsTheTokenOnceTheWorktreeStands`,
`TestClaimLeavesNoTokenWhenTheStandUpFails`,
`TestClaimWarnsWhenTheTokenCannotBeWritten` and
`TestClaimEmitsJSON` (moved onto `liveLaneHerdr`) in claim_test.go;
`TestStartPersistsTheTokenBeforeTheSessionBinds` and
`TestStartResumesAClaimOnlyLaneWithoutWaitingTheWindow` in
start_test.go; `TestReleaseEndsALaneClaimAloneStoodUp` in
release_test.go; `TestWriteTokenThenReadTokenRoundTrips` and
`TestWriteTokenReturnsTheWriteFailure` in
internal/claim/token_test.go; the `@S92` scenario in
features/lifecycle.feature, row S92 in lease-protocol.md's Lifecycle
table. Every new positive test was confirmed red against the
pre-phase tree (a WIP commit checked out, run, then restored) before
going green.

**A ripple, not a regression.** S32 ("two same-host sessions race")
in races.feature already covered a second same-host `start` meeting
its own first lane. Once that first lane's token persists immediately
— `start`'s fresh-worktree branch never bound a session in this
fixture — `holdKindFor` can now prove the hold from the token and
class it `HoldLive` off herdr's own pane list, so the second call's
refusal comes from `liveHoldRefusal`'s wording ("a live agent is on
this lane; nudge or open it instead of starting") rather than the
cruder `startLiveLaneRefusal` pre-flight check the missing token used
to fall back to. `theSecondStartIsRefusedNamingTheLane` in
`cmd/frit/bdd_host_death_and_races_test.go` now asserts the accurate
wording; the matrix's own S32 outcome — "loser's refusal names the
winning lane" — still holds, since the branch name still appears in
the message.

**Inherits to phase 2.** `holdKindFor` in `cmd/frit/dispatch.go` still
classes a lane with no persisted token — every lane a plain `git commit`/`push`
workflow stood up before this plan, or one whose write failed and was
only warned about — as `HoldUnproven`. Phase 2's own job, wording
`release` and `start`'s refusal from such a lane and carrying
`next_action`, is untouched by this phase: no verb here changes what
counts as unproven, only what a claim-only lane now proves once it
stands.

**Verified against the built frit.** `go build ./cmd/frit` plus the
full suite exercised the real code path — `liveLaneHerdr` performs an
actual `git worktree add` at the path frit supplies and leaves a real
git directory for the token to live in; a live herdr daemon happens to
be running this session's own pane, so a manual run against it was
deliberately skipped rather than risk opening a visible workspace in
the user's own terminal.

You may clear this session now; phase 2 starts fresh from
`go run ./cmd/frit phase 2609050854`.
