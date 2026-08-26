---
name: plan-phase
description: >-
  Execute one phase of a plan on a small budget: load only its
  frontmatter, Goal and the one "## Phase N" section, work red then
  green, and flip status inside the same commits that land the work.
  Trigger on "execute phase N of plan X", "continue plan X", "run the
  next phase", or "work on plan X".
---
# plan-phase

Execute exactly one phase of one plan. Every status flip rides a work
commit, so the ledger can never go stale.

## Inputs

- Plan id, or enough of the title to resolve it.
- Optional phase `N`; default is the first phase not at ✅.

## Method

1. **Load the phase.** `go run ./cmd/frit next <id>` gives the first phase not
   done — its body, tier, gate; `go run ./cmd/frit show <id>` gives the Goal
   and any blocker. Open nothing else.
2. **Honor the answers.** "already done" means stop and report, not
   redo. Honor the tier `next` names.
3. **Red then green.** Commit the failing test first, then the code
   that passes it. Verify with the narrowest instrument, then the
   phase's gate.
4. **Flip status in the same commit.**

  - First commit of the plan → plan `status:` 🔲 → 🔳.
  - A phase's closing commit → its `phases:` entry → ✅.
  - The last phase's closing commit → tick met Acceptance Criteria,
     plan `status:` → ✅, `mdsmith fix PLAN.md`. Then `mdsmith check .`
     stays clean.

5. **Stop and report** if the spec conflicts with the tree — a named
   seam is gone, a test ripples past the files it names. Do not
   improvise a design or weaken a check to reach green.
6. **Fenced** — a commit refused as "fenced" means this lane's lease
   moved under it; run `go run ./cmd/frit yield`, don't fight the CAS.
7. **Rescue conflict** — a warning naming a rescue ref means it was
   moved by hand; fetch it, inspect, delete the ref, then retry.

## Notes

- One phase per invocation. For the whole plan, loop and report
  between phases.
- A phase already passing still gets its closing commit and ✅ flip —
  say so.
- frit never edits a plan; the status flips are yours.
