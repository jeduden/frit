---
n: 2
title: release and yield agree on a decorated hold
status: "✅"
result: true
summary: >-
  release and yield now refuse a hold made of a decorated branch alone
  in the same words, naming the branch and the takeover that ends it.
  No refusal calls an unmatured hold "live" any more. C17 covers it.
---
# Phase 2 result

## Handoff

`release` no longer calls a held plan "nothing holds it" just because
no lease ref exists. A held plan with no lease ref now gets the same
refusal from `release` in its lane and `yield` outside it: "is held by
a decorated branch with no lease ref (plan/7-shader-unit); only a
takeover can end it", with the wait-or-take-over `next_action`. Both
verbs now share one decision for a hold they cannot end. So `yield`
run from a tokenless own lane also gets release's S49 wording, where
it used to say "another lane".

The refusal for an unmatured hold held by another lane now reads
"held by another lane". "Live" claimed more than an unmatured window
shows.

**Verified by.** A cmd test runs `release --json` in the decorated
lane and `yield --json` outside it, and requires the two documents to
match. It failed first on exactly the issue's pair. Unit tests pin the
new wording and the order of the shared decision. C17 drives the same
shape through godog.

**For phase 3.** The `next_action` here promises a takeover with
`frit start <id>` once the window matures. Phase 1 makes it mature;
phase 3 must make that takeover work.
