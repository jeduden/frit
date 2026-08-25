---
name: plan-sync
description: >-
  Reconcile every plan's status against git: find 🔲/🔳 plans whose
  work already landed, flip the unambiguous cases with commit
  evidence, report the rest, refresh PLAN.md. Trigger on "sync plan
  statuses", "which plans are stale", "reconcile the plans", or before
  starting work on an old plan.
---
# plan-sync

Statuses drift when work lands without its ledger flip. Walk every
not-done plan, compare it against git, repair the drift. A flip needs
direct evidence; anything less is reported, never guessed.

## Method

1. **Enumerate and gather evidence.** `go run ./cmd/frit drift --json` reports
   every not-done plan in one walk, no per-id git shell-out. Each row
   carries `landed`, `last_phase_commit` (some commit names its last
   phase), and `commits` — every commit naming its id, sha + subject.

   Classify by the strongest matching rule:

  - `landed`, and a `CLOSED`/`✅` subject or `last_phase_commit` → ✅.
  - `commits` beyond the creation commit → 🔳.
  - Only the creation commit → 🔲 stands.
  - A `claim`/`start` marker → someone is on it, not that work landed;
     it moves no status.
  - A subject naming "superseded by plan X" → candidate ⛔; report, do
     not auto-flip.

2. **Per-phase pass** for plans with `phases:`
   (`mdsmith extract plan -f json <file>`): a closing GREEN commit
   moves that phase → ✅; a RED with no GREEN stays 🔳 and is reported.
3. **Ambiguity rule.** Work landed then reverted, a tail task deferred,
   or `landed: false` → do not flip to ✅; report it with its evidence
   lines.
4. **Apply.** Edit each flipped plan's frontmatter, then
   `mdsmith fix PLAN.md` and `mdsmith check .`.
5. **Commit** all flips as one: `plans: sync statuses against git`,
   the body listing each `<id> <old>→<new>: <evidence sha + subject>`.
6. **Report** a table: id, old → new (or "unchanged" / "needs call"),
   one-line evidence. Ambiguous rows carry the question to answer.

## Notes

- Never flip ✅ back to 🔳 automatically — a human said done; question
  it in the report.
- Evidence order: explicit close commit > last-phase GREEN > phase
  commits present. Only commits are evidence, not docs or memory.
