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

Execute exactly one phase of one plan. The status ledger is part of
the work: every flip rides a work commit, so status can never go
stale.

## Inputs

- Plan id or enough of the title to resolve it.
- Optional phase `N`; default is the first phase not at ✅.

## Method

1. **Load the phase.** `frit next <id>` reports the first phase not
   done; `frit show <id>` gives the Goal and any blocker. Read the
   plan file for that phase's section, its tier and its gate — that one
   section, never the whole file by hand.
2. **Honor the two answers.** "already done" means stop and report,
   not redo. Honor the tier the Execution row names.
3. **Red then green.** Commit the failing test first, then the code
   that passes it. Verify with the narrowest instrument, then the
   phase's own gate.
4. **Flip status in the same commit.**

  - First commit of the plan → plan `status:` 🔲 → 🔳.
  - The phase's closing commit → its `phases:` entry → ✅.
  - The last phase's closing commit → tick met Acceptance Criteria,
     plan `status:` → ✅, and `mdsmith fix PLAN.md`.

   Then `mdsmith check .` must stay clean.
5. **Stop-and-report.** If the phase's spec conflicts with the tree —
   a named seam is gone, a test ripples past the files it names — stop
   and report with evidence. Do not improvise a design or weaken a
   check to reach green.

## Notes

- One phase per invocation. For "the whole plan", loop this method and
  report between phases.
- A phase whose behavior already passes still gets its closing commit
  and ✅ flip — say so in the message.
- frit never edits a plan for you; the status flips are yours to make.
