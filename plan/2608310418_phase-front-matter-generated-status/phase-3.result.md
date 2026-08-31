---
n: 3
title: plan-new writes the phase front matter and the generated catalog
status: "✅"
result: true
summary: plan-new writes {n, title, status} on each phase file and a Phases catalog in plan.md.
---
## Handoff

Plan 2608310418 closes here. The `plan-new` skill now instructs writing
`{n, title, status}` front matter on each `phase-N.md` and a `## Phases`
`<?catalog?>` in `plan.md`, and it keeps its existing "no `phases:`
ledger" line — so a new folder plan lints on the first try under the
phase-spec/phase-record requirements and gets the derived status table
for free. Existing plans keep their ledgers untouched; only new plans
skip them, until issue 111 teaches `frit doctor`/`next` the front matter.

RED was a new `internal/skills` test asserting the canonical skill names
the front-matter fields and the catalog; it failed at HEAD. GREEN edited
the canonical asset (steps 3 and 4), regenerated the dogfood copy with
the built binary — `frit skills --force --via "go run ./cmd/frit"` wrote
only `plan-new`, the four other skills byte-identical — and
`TestDogfoodCopiesMatchCanonical` stayed green. `internal/skills/assets`
carries no `proto.md`, so nothing there to mirror; the scaffold
`proto.md` was mirrored in Phase 1.

No follow-ups parked. The one deliberate scope note, recorded for the
reviewer: the optional `## Phases` section lives in every plan by way of
`proto.md`'s existing `## ...` wildcard slot, not a new required literal
— the legacy proto schema parser cannot mark a single named heading
optional, and a literal would force the section onto every plan. proto's
conventions comment documents the optional catalog. Issue 111 is the
companion that retires the transitional ledgers.
