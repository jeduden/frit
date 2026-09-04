---
name: plan-phase
description: >-
  Execute one phase of a plan on a small budget: load only its open
  phase's bundle — spec, prior handoff, notes, tier and gate — work
  red then green, and close the phase inside the same commits that
  land the work. Trigger on "execute phase N of plan X", "continue
  plan X", "run the next phase", or "work on plan X".
---
# plan-phase

Execute exactly one phase of one plan. Every status flip rides a work
commit, so the ledger can never go stale.

## Inputs

- Plan id, or enough of the title to resolve it.
- Optional phase `N`; default is the first phase not at ✅.

## Method

1. **Load the phase.** `go run ./cmd/frit phase <id>` bundles the open phase in
   one call: its spec, tier, gate, the previous phase's handoff, and
   any notes already parked, plus the result file to write when the
   plan carries per-phase files. `go run ./cmd/frit show <id>` gives the Goal
   and any blocker. Open nothing else.
2. **Honor the answers.** "already done" means stop and report, not
   redo. A plan already held live (`held: true`, `dead: false`) is
   already running somewhere — report it, never start a second runner
   there. Honor the tier `phase` names.
3. **Red then green.** Commit the failing test first, then the code
   that passes it. Verify with the narrowest instrument, then the
   phase's gate. Park a follow-up or a side quest in the result file
   `phase` named rather than chasing it.
4. **Close the phase with `/plan-handoff`**, riding this same commit.
   It writes the handoff in the plan's shape and flips the phase's
   status — the first commit of a plan also moves it 🔲 → 🔳, the last
   phase's close ticks the met Acceptance Criteria and moves it → ✅.
   Then `mdsmith check .` stays clean.
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
- A phase already passing still gets its closing commit and its close
  — a `## Handoff` or a ✅ flip, whichever the plan uses — say so.
- frit only reads; every plan and phase file you write is yours.
