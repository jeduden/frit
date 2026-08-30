---
name: plan-new
description: >-
  Create a plan folder under plan/ that passes mdsmith on the first
  try: minute-precision id, plan.md's Goal/Context/Tasks/Execution/
  Acceptance Criteria, each phase in its own phase-N.md, and a
  refreshed PLAN.md. Trigger on "plan this work", "create a plan",
  "new plan for X", or "capture this as a plan".
---
# plan-new

Create one plan folder under `plan/`, conforming to `plan/proto.md`.
frit reads these files but never writes them, so this is mdsmith and
an editor, not a frit verb.

## Method

1. **Id.** `date -u +%y%m%d%H%M`. If `plan/<id>_*` exists, add a
   minute and retry. It is both the frontmatter `id:` and the folder
   name: `plan/<id>_<kebab-slug>/plan.md`, with each phase's own
   `phase-N.md` spec beside it. A trivial, single-phase plan may stay
   flat instead — `plan/<id>_<kebab-slug>.md`, `## Tasks` alone.
2. **Reuse first.** Search for machinery that already does what the
   plan needs; `## Context` names what was searched and why each
   candidate was or was not reused.
3. **Phase 1 is a proving slice** — a minimal end-to-end slice that
   demonstrates the Goal and fixes the test approach later phases copy.
   Write it as `phase-1.md`: freeform prose under the `phase-spec`
   kind, its RED spec, GREEN sites and gate. Declare Phase 1 alone; a
   later phase's `phase-N.md` is added once the prior phase's Handoff
   shows the real shape.
4. **Write `plan.md`** to the `plan/proto.md` shape: frontmatter
   (`id`, `title`, `status: "🔲"`, `summary`, `model`, `depends-on` —
   no `phases:` ledger, since each phase's own result file carries its
   state), then `## Goal`, `## Context`, `## Tasks`, `## Execution`,
   `## Acceptance Criteria`.
5. **Tier per phase** in the Execution table: the cheapest tier a loud
   gate makes safe. Design stays opus; implementing from a written
   assertion is cheap. Set frontmatter `model:` to the dominant
   implement tier.
6. **Lint and index.** `mdsmith check plan/<id>_<slug>` (fix line
   length 80 and long sentences), `mdsmith fix PLAN.md`, then
   `mdsmith check .`.
7. **Health-check.** `go run ./cmd/frit doctor` scans every plan on disk; find
   this id in its output — no missing Goal, no Execution row short of a
   phase, no tier `plan/proto.md` rejects. Fix the plan, not the check.
8. **Commit** the plan folder and PLAN.md together: `plan <id>: <title>`.

## Notes

- Never renumber phases — append. `depends-on:` lists plan ids this
  plan cannot start before.
- A folder plan sits one directory deeper than a flat plan: a repo
  path link copied from a flat plan needs one more `../`.
- A phase that ships or edits a skill gates on the claim, not the copy:
  run the command against the built `frit` and confirm the output
  matches. Lint and `TestDogfoodCopiesMatchCanonical` pass on a false
  claim.
