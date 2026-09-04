---
n: 5
title: plan-phase fronts plan-handoff for the close
status: "🔳"
result: false
---
Task 5, the plan's last. `plan-phase`'s step 4 restates the whole
close recipe inline — the 🔲 → 🔳 first-commit flip, the phase-file
`{n, title, status, result: true, summary}` front matter and its `##
Handoff`, the ledger `phases:` flip, the last-phase Acceptance Criteria
tick and `status:` → ✅. `plan-handoff` (phase 1) now owns that recipe
in full. Two copies of the same instructions drift. Point `plan-phase`
step 4 at `/plan-handoff` and let the one skill carry the recipe.

**Assumes.** `plan-handoff`'s SKILL.md already covers every branch
`plan-phase` step 4 restates. It carries both plan shapes and the
first-commit `status:` flip. It also carries the last phase's criteria
tick and `mdsmith fix PLAN.md`. So step 4 loses nothing by delegating.
The canonical asset is
[internal/skills/assets/plan-phase/SKILL.md](../../internal/skills).
Its dogfood copy under `.claude/skills` is regenerated, not
hand-edited, and `TestDogfoodCopiesMatchCanonical` pins the two
together.

**RED.** In
[internal/skills/skills_test.go](../../internal/skills/skills_test.go),
add `TestPlanPhaseFrontsPlanHandoffForTheClose`. It reads
`assets/plan-phase/SKILL.md` and asserts the body names `plan-handoff`
— step 4 now points at the skill — and no longer restates the recipe
inline, checked by the absence of the front-matter detail `result:
true, summary` that now lives only in `plan-handoff`. It fails first:
today step 4 carries that detail and never names `plan-handoff`.

**GREEN.** Three changes:

- Rewrite `plan-phase` step 4 in the canonical
  `internal/skills/assets/plan-phase/SKILL.md` to close the phase with
  `/plan-handoff`, keeping only what `plan-phase` itself gates: the
  close rides the same commit as the work, and `mdsmith check .` stays
  clean afterward. The per-shape recipe moves out to `plan-handoff`.
- Regenerate the dogfood copy: `{{frit}} skills . --via "go run
  ./cmd/frit" --force`, so `.claude/skills/plan-phase/SKILL.md` matches
  and `TestDogfoodCopiesMatchCanonical` passes.
- Confirm the `skill` kind's token budget still passes — the pointer is
  shorter than the recipe it replaces, so the budget only loosens.

**Gate.** `go test ./internal/skills/...` covers the new assertion and
the dogfood match. Full `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` stay clean. `mdsmith
check .` clean, the `skill` kind's token budget met. This is the last
phase: its close ticks the met Acceptance Criteria and moves the plan
`status:` → ✅.
