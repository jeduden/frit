---
n: 1
title: frit phase finds the open phase and emits the working bundle
status: "✅"
result: true
summary: frit phase now bundles the open phase — spec, prior handoff, notes, tier, gate and the result path — in one call.
---
## Handoff

Phase 1 landed. `frit phase <id>`, run inside a plan's own lane, now
bundles the open phase — spec, previous handoff, parked notes, tier,
gate, and the result file to write — in one call.

Shipped:

- `planmeta.Resume` (internal/planmeta/resume.go): globs a folder
  plan's `phase-N.md` files, ordered numerically then by full token
  (so `3a` precedes `3b` precedes `10`), and returns the first whose
  `phase-N.result.md` is absent or carries no top-level `## Handoff`
  heading — parsed via the AST, not a substring, so a fenced or
  quoted mention doesn't count. Tier and gate come from the plan's
  `## Execution` table directly, independent of any `phases:` ledger.
  An empty `dir` (a flat plan, or a folder plan with no `phase-N.md`)
  falls back to the `phases:` ledger and `## Phase N` sections
  unchanged — this is exactly that path, since this plan itself still
  carries only a ledger entry for phase 1.
- `report.PhaseDoc`/`NewPhase` (internal/report/phase.go) carry the
  bundle into the JSON contract.
- `frit phase` (cmd/frit/main.go): resolves the plan, requires the
  cwd stand in that plan's own lane (refuses otherwise — phase files
  live only in a worktree), reads plan.md, and only passes a
  directory to `Resume` when `plans.IsFolderPlanFile(plan.Path)` — a
  flat plan's path parent is the shared `plan/` directory, never
  globbed, so a stray or mislaid `phase-N.md` there can't be
  misattributed to it.
- `plan-phase` skill folded the verb in: step 1 now loads via one
  `frit phase` call instead of `next`; step 4 closes a phase-file
  plan by writing its result file's `## Handoff` instead of flipping
  a `phases:` entry.

A `/code-review --fix` pass after landing found and fixed the flat-plan
misattribution above, added alphanumeric (`3a`/`3b`) phase-file
support to match the ledger's own convention, dropped an untested
defensive branch, and restored a ledger phase's title (dropped when
step 1 stopped calling `next`). Three lower-value findings (a
duplicate lane-resolution call that mirrors existing precedent in
`laneOverride`, a cheap double AST walk, and a struct the alphanumeric
fix made load-bearing) were left as-is with rationale recorded via
`ReportFindings`.

Verified: the three RED cases from the phase spec, the review-fix
follow-ups, `go test ./...`, `go vet ./...`, golangci-lint, and
`mdsmith check .` are all clean as of this handoff.

**What phase 2 inherits:** Tasks 2–4 are undetermined — the plan's own
Tasks list marks them "(determined after Phase 1)". Candidates named
in the Goal and Context sections, not yet split into phases: `plan-new`
authoring a folder plan with `phase-N.md`/`phase-N.result.md` by
default; the two new mdsmith kinds (spec, record) with their
`required-structure` schema and token budget, narrowing the freeform
`plan/*/*.md` override to exclude them, alongside the `plan` kind's
own schema loosening; the mdsmith bump to v0.55.1 (go.mod and the CI
action pin together); and headroom's retirement once `plan.md` no
longer grows with phases. Phase 2 should split these into concrete,
independently gated slices the way Phase 1 did — start with `frit
next <this-id>` or read the plan's own Context section rather than
re-deriving the above from git log.
