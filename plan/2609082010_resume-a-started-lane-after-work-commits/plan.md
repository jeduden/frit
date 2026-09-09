---
id: 2609082010
title: Resume a started lane after work commits advance its token
status: "✅"
summary: >-
  Reproduce issue #186 through normal start, binding and token persistence,
  then restore same-epoch resume after ordinary pushed commits. Preserve
  the checkout and the live-agent and foreign-epoch guards. S94 proves
  the full dispatch-to-resume path.
model: sonnet
depends-on: []
---
# Resume a started lane after work commits advance its token

## Goal

A clean, unattended lane with a valid persisted token resumes through
`frit start <id> --go` after its work advances origin, without waiting
for takeover. Addresses [issue #186][issue].

[issue]: https://github.com/jeduden/frit/issues/186

## Context

The issue reports an existing clean worktree, a persisted beat token,
an origin tip descending from it, and no live agent. Starting from
inside that lane still gives the takeover-window refusal. The cause
is unconfirmed; ancestry alone does not prove the current epoch.

The completed [resume plan][resume] covers finding a token from outside
the lane. The completed [raw-commit plan][raw] covers recognizing work
beyond a token. Neither is an open plan for this regression.

[resume]: ../2609011836_resume-a-held-lane-you-own/plan.md
[raw]: ../2608231006_release-recognizes-own-lane-after-raw-commits.md

Reuse the lease fixtures and herdr fake in
[start tests](../../cmd/frit/start_test.go), including the persisted-token
and own-commit resume tests. They already cover hand-built leases; the
new regression must begin with normal dispatch and its persisted beat.
Trace lane resolution and token proof in
[claim](../../cmd/frit/claim.go) and [start](../../cmd/frit/start.go),
then marker lookup in [the lease](../../internal/claim/lease.go).

Also test ordinary subjects beginning `plan <id>:`. Existing raw-commit
resume tests use `red:` and `green:` subjects, while marker lookup first
selects a matching plan prefix. A work commit could mask a valid marker;
this is a hypothesis to drive red, not an established issue cause.

S76 proves outside-lane resume and S86 exercises own advancement through
claim/release. Add S94 in [cross-layer scenarios][feature] for a normal
start followed by an in-lane start after work commits. Keep the existing
guards against a live agent, missing proof and a foreign epoch.

[feature]: ../../features/cross-layer.feature

## Tasks

1. Reproduce the refusal through normal dispatch, pushed work and an
   absent agent; identify the first failed proof and pin it in a test.
2. Repair that proof path while retaining token, epoch and liveness
   checks. Prove same-worktree resume and preservation of committed work.
3. Implement S94's steps, remove its pending labels, and cite the
   scenario in [claiming](../../docs/claiming.md).

## Execution

| Phase | Title                                        | Tier   | Gate                                                                                                                                       |
| ----- | -------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | Resume after normal dispatch and pushed work | sonnet | S94 fails before the fix and passes after; built frit resumes the same checkout with the same epoch; guard regressions and full tests pass |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                   |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [Resume after normal dispatch and pushed work](phase-1.md)                                                                                                                                              |
|     | ↳      | Reproduced issue #186 through a real CLI `start --go` dispatch, then fixed marker lookup so a resume's ownership proof walks past a work commit that only shares the marker's own "plan <id>: " prefix. |
<?/catalog?>

## Acceptance Criteria

- [x] Starting normally, pushing ordinary work and closing the agent
      leaves a lane that `start <id> --go` resumes before the window.
- [x] Both plain and `plan <id>:` work subjects retain valid token proof.
- [x] Resume preserves the same checkout and pushed commits, renews the
      same epoch from the current origin tip, and dispatches one agent.
- [x] Dry-run describes resume without changing origin or starting an
      agent; JSON reports resume and the dispatched pane after `--go`.
- [x] Missing or invalid tokens, a foreign epoch, and a live agent
      cannot gain the resume shortcut; existing uncertainty guards pass.
- [x] S94 runs without `@pending`; S76, S77 and S86 remain green.
- [x] `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`
      and `mdsmith check .` pass.
