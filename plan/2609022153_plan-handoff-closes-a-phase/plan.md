---
id: 2609022153
title: A plan-handoff skill closes a phase in one command
status: "✅"
summary: >-
  Closing a phase is pure agent discipline today: plan-phase reminds
  the executor to record a handoff, but frit produces nothing and
  checks nothing, so a handoff lands in memory or a stray file, or in a
  shape frit's resume cannot read. This plan makes the close a
  one-command skill, plan-handoff, that records the handoff in the
  shape the plan uses — a `## Handoff` heading in a single-file plan, a
  separate phase-N.result.md in a directory plan — always in a form
  frit reads back, flips the phase's status inside the closing commit,
  and ends with a safe-to-clear cue. Resume then surfaces a single-file
  plan's handoff too, and a doctor guard catches a phase recorded done
  whose handoff is missing in either shape.
model: sonnet
depends-on: []
---
# A plan-handoff skill closes a phase in one command

## Goal

Closing a phase is one command, `/plan-handoff`. It records the
handoff in the shape the plan uses — a `## Handoff` heading in a
single-file plan, a separate `phase-N.result.md` in a directory plan —
always in a form frit's resume reads back. It flips the phase's status
inside the same commit that lands the work, and cues a clean session
start. A skipped or unreadable handoff no longer passes unseen: a
doctor check reports it.

## Context

**The gap.** The [plan-phase skill](../../.claude/skills/plan-phase/SKILL.md)
step 4 leaves the close to the executor's memory: a directory plan
writes `phase-N.result.md`, a single-file plan just flips its
`phases:` entry. frit only prints a reminder — `printPhase` in
[cmd/frit/main.go](../../cmd/frit/main.go) ends with "Write the handoff
to phase-N.result.md" — then produces nothing and checks nothing. So
the handoff lands wherever the session puts it: in the agent's memory,
an ad-hoc note, or a shape frit cannot read.

**Two shapes, one command.** A handoff is reported the way the plan is
laid out. A single-file plan, `plan/<id>_<slug>.md`, carries a `##
Handoff` heading inline. A directory plan,
`plan/<id>_<slug>/plan.md`, carries a separate `phase-N.result.md`
beside it. `/plan-handoff` picks the target from the plan's shape, so
the executor never chooses the wrong home for it.

**The shape frit can read is narrow.** `handoffOf` in
[internal/planmeta/resume.go](../../internal/planmeta/resume.go) finds
a handoff only at a level-2 heading whose exact text is `Handoff` —
never a substring, never a bold `**Handoff.**` lead. A directory plan's
result file that closes with bold prose resumes as if it left no
handoff. This already happened: the phase-1 result in
[plan 2609021554](../2609021554_gather-reports-progress-and-status/plan.md)
ends `**Handoff.** …`. And `resumeFromLedger` carries no handoff at
all, so a single-file plan's `## Handoff` never surfaces on resume even
when it is written.

**Fix the workflow, teach resume, then guard the skip.** A skill closes
the discipline gap the way
[plan 2608212223](../2608212223_a-skill-fronts-every-verb.md) fronts
every verb: one command instead of a recipe reconstructed from memory.
Resume then reads a single-file plan's `## Handoff` as the prior
phase's context, closing the readback gap. A doctor check makes a skip
visible after the fact, the shape `checkPhaseNumberSync` in
[internal/doctor/doctor.go](../../internal/doctor/doctor.go) already
uses to point one finding at the file that carries it.

**The guard must see the lane, not just main.** A doctor that only
reads the fleet's default-branch checkout catches a skip long after the
phase has merged. It never catches it at the close, in the lane, where
the executor could still fix it. Worse, it silently disagrees with
`next`, `show` and `phase`. Those already swap in the lane's own
working copy when run inside it — the `laneOverride` seam in
[cmd/frit/main.go](../../cmd/frit/main.go). A blind-agent test of the
main-only behavior — four agents, two of them smaller models — came
back unanimous. The "fix it, re-run, still reported" gap reads as *my
fix didn't take*, and drives a re-fix loop. It is worst for the smaller
models, which act on the literal signal. So doctor honors the lane it
runs in, the same as its siblings. The plan the cwd stands in is
checked from the lane's copy; every other plan reads the fleet's
default branch.

**Reuse first.** The skill rides the existing bundle: canonical text in
[internal/skills/assets](../../internal/skills), laid down by `frit
skills`, dogfood copies guarded by `TestDogfoodCopiesMatchCanonical`,
and the `skill` kind's 650-token budget in
[.mdsmith.yml](../../.mdsmith.yml). No new machinery. Resume and the
guard both reuse `handoffOf`, which already decides whether a `##
Handoff` heading is present, so neither re-derives the parsing. The
guard reuses `doctor.Finding` and the per-file finding shape from
`checkPhaseNumberSync`. The safe-to-clear cue reuses the token-cheap
resume design of
[plan 2608300937](../2608300937_per-phase-files-token-cheap-resume/plan.md).
Once the handoff is durable, the session's working context is
disposable, and the next phase reads only the bundle `frit phase`
assembles.

## Tasks

1. Phase 1 (proving slice): add the `plan-handoff` skill to the bundle
   and regenerate frit's own dogfood copy. The skill picks the handoff
   target from the plan's shape — a `## Handoff` heading in a
   single-file plan's `plan.md`, a separate `phase-N.result.md` in a
   directory plan — flips the phase's status inside the commit that
   lands the work, and closes with a safe-to-clear cue. Name it in the
   bundle list in
   [internal/skills/skills.go](../../internal/skills/skills.go) and in
   [docs/development.md](../../docs/development.md).
2. Later, once the handoff shows the shape: teach resume to read a
   single-file plan's `## Handoff` heading as the prior phase's handoff
   in
   [internal/planmeta/resume.go](../../internal/planmeta/resume.go), so
   `frit phase` surfaces it the way it already surfaces a directory
   plan's result-file handoff.
3. Later: a doctor check reports a phase recorded done whose handoff is
   missing in its plan's shape — no readable `## Handoff` in a
   single-file plan, none in a directory plan's `phase-N.result.md`.
   Fix the existing drift in the phase-1 result under
   [plan 2609021554](../2609021554_gather-reports-progress-and-status/plan.md)
   so a real run of the check comes back clean on it.
4. Later: `frit doctor` honors the lane it runs in. When the cwd
   stands in a plan's own lane, that plan is checked from the lane's
   working copy — so a handoff written in the lane clears the finding
   before the branch merges — the way `next`, `show` and `phase`
   already read the lane via `laneOverride` in
   [cmd/frit/main.go](../../cmd/frit/main.go). Every other plan still
   reads the fleet's default-branch copy.
5. Later: point [plan-phase](../../.claude/skills/plan-phase/SKILL.md)
   step 4 at `/plan-handoff` for the close rather than restating the
   recipe inline, so the two skills cannot drift.

## Execution

| Phase | Title                                                | Tier   | Gate                                                                                                                                              |
| ----- | ---------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The plan-handoff skill in the bundle                 | sonnet | built `frit skills --via "go run ./cmd/frit"` lays plan-handoff into `.claude/skills`; `mdsmith check .` clean, token budget met                  |
| 2     | Resume surfaces a single-file plan's Handoff heading | sonnet | `go test ./internal/planmeta/...`; full `go test ./...` and lint clean                                                                            |
| 3     | A doctor check catches a skipped handoff             | sonnet | `go test ./internal/planmeta/... ./internal/doctor/...`; full `go test ./...` and lint clean; a built `frit doctor` over this repository is clean |
| 4     | doctor honors the lane it runs in                    | sonnet | `go test ./internal/doctor/... ./cmd/frit/...`; full `go test ./...` and lint clean                                                               |
| 5     | plan-phase fronts plan-handoff for the close         | sonnet | `go test ./internal/skills/...`; full `go test ./...` and lint clean; `mdsmith check .` clean, token budget met                                   |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The plan-handoff skill in the bundle](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
|     | ↳      | plan-handoff joined the bundle — internal/skills/assets/plan-handoff and its dogfooded .claude/skills copy — teaching the split close: a `## Handoff` heading for a single-file plan, a phase-N.result.md for a directory plan, either way riding the commit that lands the work.                                                                                                                                                                                                                     |
| 2   | ✅     | [Resume surfaces a single-file plan's Handoff heading](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                    |
|     | ↳      | resumeFromLedger now calls handoffOf(planBody) and carries a single-file plan's top-level `## Handoff` heading as the open phase's HandoffIn, the same field a directory plan's result file already filled — frit phase surfaces it identically either way.                                                                                                                                                                                                                                           |
| 3   | ✅     | [A doctor check catches a skipped handoff](phase-3.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                |
|     | ↳      | frit doctor now reports a "handoff" finding for a phase recorded done in a plan still in progress whose handoff has no readable trace: no "## Handoff" in a single-file plan, none in a directory plan's own phase-N.result.md. A plan already done or superseded is exempt, since nothing resumes into it — a 34-plan survey of this repository confirmed every already-`✅` plan stays clean under that rule. The one live gap, plan 2609021554's phase-1 result closing with bold prose, is fixed. |
| 4   | ✅     | [doctor honors the lane it runs in](phase-4.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|     | ↳      | frit doctor now reads the plan whose lane the cwd stands in from the lane's own working copy, so a gap fixed in the lane clears before the branch merges — the same narrowing next, show and phase already apply via laneOverride. Every other plan still reads the fleet's default-branch copy. New doctor.ScanID re-checks one plan from a given root; doctorCmd.Run swaps that plan's findings when repo.Name matches the current lane; the help text now states the lane read.                    |
| 5   | ✅     | [plan-phase fronts plan-handoff for the close](phase-5.md)                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|     | ↳      | plan-phase step 4 now closes the phase with /plan-handoff instead of restating the recipe inline, so the one skill owns the close and the two cannot drift. The canonical asset and its regenerated dogfood copy match, the skill kind's token budget still passes, and this closes the plan.                                                                                                                                                                                                         |
<?/catalog?>

## Acceptance Criteria

- [x] `/plan-handoff` records the handoff in the plan's shape: a `##
      Handoff` heading in a single-file plan's `plan.md`, a separate
      `phase-N.result.md` in a directory plan — each inside the commit
      that lands the phase's work
- [x] The built `frit skills --via "go run ./cmd/frit"` lays
      `plan-handoff` into `.claude/skills`, and the skill passes the
      `skill` kind's 650-token budget
- [x] `frit phase` on a single-file plan surfaces the prior phase's
      `## Handoff` as its inherited handoff, the way it already does for
      a directory plan's result file
- [x] `frit doctor` reports a phase recorded done whose handoff is
      missing in its plan's shape, and comes back clean once the
      existing drift is fixed
- [x] Run inside a plan's own lane, `frit doctor` checks that plan from
      the lane's working copy — a handoff written in the lane clears the
      finding before the branch merges — the way `next`/`show`/`phase`
      already read the lane; every other plan still reads the fleet's
      default-branch copy
- [x] `plan-phase`'s step 4 points at `/plan-handoff` for the close
      rather than restating the recipe inline, and its dogfood copy
      matches the canonical asset (`TestDogfoodCopiesMatchCanonical`)
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
