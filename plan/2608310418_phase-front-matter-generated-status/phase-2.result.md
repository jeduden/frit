---
n: 2
title: A phase record requires the front matter and the existing records migrate
status: "✅"
---
## Handoff

The `phase-record` kind now requires `{n, title, status}` (MDS071),
matching the `phase-spec` requirement from Phase 1. RED proved it: with
the rule added, folder plan 2608300937's two records — `phase-1.result.md`
and `phase-3.result.md` — failed for the missing keys, while this plan's
own record (migrated early in Phase 1) stayed clean.

GREEN migrated those two records. Each gained front matter matching that
plan's ledger truth: `phase-1.result.md` is `n: 1`, its ledger title,
`✅`; `phase-3.result.md` is `n: 3`, the title its `phase-3.md` spec
already carries, `✅`. That plan's `phases:` ledger was NOT touched —
`frit doctor`/`next` still read it, and issue 111 retires it later.

Phase 3 inherits: teach the `plan-new` skill to write phase front matter
and the `## Phases` catalog, and to STOP writing the `phases:` ledger for
NEW plans (existing ledgers stay). Update BOTH copies —
`internal/skills/assets/plan-new/SKILL.md` and the dogfood
`.claude/skills/plan-new/SKILL.md` — and keep them in sync
(`TestDogfoodCopiesMatchCanonical`). Gate on the built `frit skills`
claim writing the dogfood copy, not on lint alone. Both `.mdsmith.yml`
phase kinds now require the front matter, so any plan-new-authored phase
file that omits it will fail the linter — the skill must write it.
