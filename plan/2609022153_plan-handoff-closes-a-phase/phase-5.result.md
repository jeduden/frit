---
n: 5
title: plan-phase fronts plan-handoff for the close
status: "✅"
result: true
summary: >-
  plan-phase step 4 now closes the phase with /plan-handoff instead of
  restating the recipe inline, so the one skill owns the close and the
  two cannot drift. The canonical asset and its regenerated dogfood
  copy match, the skill kind's token budget still passes, and this
  closes the plan.
---
## Handoff

The plan's last phase. `plan-phase`'s step 4 was five lines of
close recipe — the 🔲 → 🔳 first-commit flip, the phase-file front
matter and its `## Handoff`, the ledger `phases:` flip, the last-phase
Acceptance-Criteria tick. `plan-handoff` (phase 1) already carries all
of it. Step 4 now reads "Close the phase with `/plan-handoff`, riding
this same commit," keeping only what `plan-phase` itself gates: the
close rides the work commit, and `mdsmith check .` stays clean after.
The recipe lives in one place; the two skills cannot drift.

RED was `TestPlanPhaseFrontsPlanHandoffForTheClose` in
`internal/skills/skills_test.go`: the plan-phase asset must name
`plan-handoff` and must no longer carry the `result: true, summary`
front-matter detail that now lives only in `plan-handoff`. It failed
first — step 4 restated the recipe and never named the skill.

GREEN edited the canonical
`internal/skills/assets/plan-phase/SKILL.md`, then regenerated the
dogfood copy with `frit skills . --via "go run ./cmd/frit" --force`, so
`.claude/skills/plan-phase/SKILL.md` matches and
`TestDogfoodCopiesMatchCanonical` passes. The pointer is shorter than
the recipe it replaced, so the `skill` kind's token budget only
loosens.

**Plan complete.** All five phases are ✅ and the plan is ✅. Closing a
phase is one command, `/plan-handoff`, which records the handoff in the
plan's shape and flips the phase's status in the same commit; resume
surfaces a single-file plan's `## Handoff` too; `frit doctor` catches a
skipped handoff and — run inside a lane — reads that plan from the
lane's own copy so the guard fires at the close, before merge; and
`plan-phase` now fronts `plan-handoff` for the close rather than
carrying a second copy of the recipe.

One cross-plan artifact outlives this lane: `frit doctor` still reports
plan 2609021554's handoff gap until this plan merges to main, because
the fix to that plan lives on this branch and doctor reads every other
plan from the default branch. That resolves on merge — it is the same
lane boundary next, show and phase keep.
