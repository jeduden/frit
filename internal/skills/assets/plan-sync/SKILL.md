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
not-done plan, compare it against git, and repair the drift. A flip
needs direct evidence; anything less is reported, never guessed.

## Method

1. **Enumerate.** `mdsmith list query 'status: "🔲"' plan/` and again
   for `"🔳"`. `{{frit}} plans` reads each plan's status off every ref
   for cross-lane context.
2. **Gather evidence per plan.** The id is the filename prefix.

   ```sh
   git log --all --oneline --grep="<id>"
   ```

   Classify by the strongest matching rule:

  - `CLOSED`, `✅`, or a GREEN commit for the last phase → ✅.
  - Any phase or task work beyond the plan's creation commit → 🔳.
  - Only the creation commit → 🔲 stands.
  - A `claim`/`start` marker → someone is on it, not that work
     landed; it moves no status.
  - "superseded by plan X" → candidate ⛔; report, do not auto-flip.

3. **Per-phase pass** for plans with `phases:`
   (`mdsmith extract plan -f json <file>`): a closing GREEN commit
   moves that phase → ✅; a RED with no GREEN stays 🔳 and is
   reported.
4. **Ambiguity rule.** Work landed then reverted, a tail task
   deferred, or evidence only on an unmerged branch → do not flip to
   ✅. Report it with its evidence lines instead.
5. **Apply.** Edit each flipped plan's frontmatter, then
   `mdsmith fix PLAN.md` and `mdsmith check .`.
6. **Commit** all flips as one: `plans: sync statuses against git`,
   the body listing each `<id> <old>→<new>: <evidence sha + subject>`.
7. **Report** a table: id, old → new (or "unchanged" / "needs call"),
   one-line evidence. Ambiguous rows carry the question to answer.

## Notes

- Never flip ✅ back to 🔳 automatically — a human said done; question
  it in the report.
- Evidence order: an explicit close commit > last-phase GREEN > phase
  commits present. Only commits are evidence, not docs or memory.
