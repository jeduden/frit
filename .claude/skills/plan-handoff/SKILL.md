---
name: plan-handoff
description: >-
  Close a phase in one command: write its handoff in the shape the
  plan uses, flip its status inside the commit that lands the work,
  and cue a clean session start. Trigger on "hand off phase N", "close
  the phase", "write the handoff", "wrap up before a new session".
---
# plan-handoff

Closing a phase is discipline, not memory. This writes the handoff
where `go run ./cmd/frit phase` reads it back, so the next session inherits it
rather than reconstructing it.

## Method

1. **Find the plan's shape.** A single file `plan/<id>_<slug>.md`
   carries every phase inline. A directory
   `plan/<id>_<slug>/plan.md` carries each phase in its own
   `phase-N.md`, closed by a sibling `phase-N.result.md`.
2. **Single-file plan.** Write or replace the `## Handoff` heading in
   `plan.md`: the outcome, and what the next phase inherits. Flip the
   closed phase's `phases:` entry to ✅.
3. **Directory plan.** Write `phase-N.result.md` with front matter
   `{n, title, status: "✅", result: true, summary}` and a `##
   Handoff` section — the exact heading; a bold `**Handoff.**` lead or
   a substring match does not resume. Flip the matching `phase-N.md`
   `status:` to ✅.
4. **Commit the close riding the work.** The first commit of a plan
   also flips its `status:` 🔲 → 🔳. The last phase's close also ticks
   the met Acceptance Criteria and moves `status:` → ✅, then `mdsmith
   fix PLAN.md`.
5. **Cue the clear.** The handoff is now durable in the repo — end by
   telling the session it may be cleared; the next phase starts fresh
   from only what `go run ./cmd/frit phase <id>` assembles.

## Notes

- Skip this and a later phase inherits nothing: resume reads the `##
  Handoff` heading, never a session's memory.
- `go run ./cmd/frit doctor` reports a phase recorded done whose handoff is
  missing in its plan's shape — run it if a close looks skipped.
