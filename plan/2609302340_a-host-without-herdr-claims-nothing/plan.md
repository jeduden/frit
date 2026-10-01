---
id: 2609302340
title: A host without herdr claims nothing it would have to unwind
status: "🔳"
summary: >-
  On a host with no herdr installed, claim, start --go and pick --go
  minted a lease, failed to stand the lane up, and pushed a release:
  two writes to origin for an outcome certain before the first. They
  now refuse locally before the mint. A second phase gives such a
  host — a cloud agent session, whose branch the harness fixes — a
  way to hold a plan at all, once the lane shape is decided.
model: opus
depends-on: []
---
# A host without herdr claims nothing it would have to unwind

## Goal

A session on a host with no herdr learns that before anything reaches
origin, and — once phase 2 lands — can still hold a plan from the
checkout it already has.

## Context

**What happened.** A Claude Code cloud container has no herdr on
`$PATH`. There, `frit claim <id>` pushed `plan/<id>` with a claim
marker. Standing the worktree up then failed with `exec: "herdr":
executable file not found`, and the unwind pushed a release marker.
No lease was left dangling, but origin gained a work ref holding only
a claim and a release. `start --go` and `pick --go` share the same
mint-then-stand-up path and did the same. Reported against v0.14.0
from jeduden/cairn.

**Certain versus unreachable.** A missing binary makes every herdr
call certain to fail. A herdr that is installed but whose socket
refuses a dial is not certain: its server may come up, and a fake
herdr that fails `agent list` but answers `worktree create` is a
deliberately pinned shape (the fail-open live-lane check in
[start_test.go](../../cmd/frit/start_test.go)). The lease protocol's
S60 and S61 rows also drive an unreachable herdr through the mint and
its unwind, as a cross-layer race. So the preflight refuses only the
certain case — the executable not found — and leaves an unreachable
socket to the existing unwind.

**Where the gate sits.** Last, just before the mint. A plan claim
would refuse anyway — held, blocked, done — keeps its own, more useful
reason on a headless host. A dry-run `start` mints nothing, so it is
not gated. `pick --go` reports the refusal on its top candidate rather
than skipping to the next, since every candidate would meet the same
missing herdr.

**The herdr-less lane.** The issue's second ask is a way to hold a
plan with no worktree and no pane, from the checkout a cloud session
already runs in. That runs against the rule that a claimed lane is its
own worktree, never the shared clone (plan 2608192322), and against
how a lane is recognised. Phase 2 lays out the decisions it needs.

## Tasks

1. Phase 1: refuse claim, `start --go` and `pick --go` locally when
   herdr is not installed, before anything is pushed.
2. Phase 2: decide the herdr-less lane's shape, then build it.

## Execution

| Phase | Title                                            | Tier | Gate                                                                                                                                              |
| ----- | ------------------------------------------------ | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Refuse before the mint when herdr is not present | opus | C15 and C16 run the built frit under a `$PATH` holding git alone: refused naming herdr, origin gains no work ref; `go test ./...` green           |
| 2     | A lane with no worktree and no pane              | opus | the owner's decisions recorded in the phase; the built frit claims from a herdr-less checkout, resumes and releases it, and reap never removes it |

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

| #   | Status | Phase                                                                                                                                                                                           |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [Refuse before the mint when herdr is not present](phase-1.md)                                                                                                                                  |
|     | ↳      | claim, start --go and pick --go refuse locally on a host with no herdr installed, naming it and pushing nothing; an unreachable but installed herdr keeps the old unwind. C15 and C16 cover it. |
| 2   | 🔲     | [A lane with no worktree and no pane](phase-2.md)                                                                                                                                               |
<?/catalog?>

## Acceptance Criteria

- [x] On a host with no herdr installed, `frit claim` refuses with
      `herdr not found; nothing claimed`, and origin gains no work ref.
- [x] `start --go` and `pick --go` refuse the same way; `pick --go`
      claims no candidate.
- [x] A plan claim would refuse anyway keeps its own reason on such a
      host, and a dry-run `start` still composes.
- [x] A herdr that is installed but unreachable keeps the existing
      mint-and-unwind (S60, S61 unchanged).
- [ ] A session with no herdr can hold a plan from its own checkout,
      per the decisions phase 2 records.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
