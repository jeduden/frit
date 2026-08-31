---
n: 2
title: A phase record requires the front matter and the existing records migrate
status: "✅"
---
Phase 1 made the `phase-spec` kind require `{n, title, status}`. Do the
same for the `phase-record` kind — the `phase-N.result.md` living
record — so a result file carries the same load-bearing status as its
spec. Then migrate every existing record so `mdsmith check .` stays
clean.

Assumes: the `phase-record` kind in [.mdsmith.yml](../../.mdsmith.yml)
pins only the filename today. The `phase-spec` kind did too, before
this plan. The one pre-existing folder plan
[2608300937](../2608300937_per-phase-files-token-cheap-resume/plan.md)
holds two records with no front matter: `phase-1.result.md` and
`phase-3.result.md`. This plan's own `phase-1.result.md` already has
front matter, added early in Phase 1.

Value: a phase record is the artifact whose `## Handoff` closes a phase,
so its status belongs in the same generated home the spec now uses. With
both kinds requiring the front matter, a folder plan's `## Phases`
catalog could later read either file; more immediately, the linter
guards every per-phase file the same way.

RED. In [.mdsmith.yml](../../.mdsmith.yml), add
`required-frontmatter: [n, title, status]` to the `phase-record` kind,
mirroring the `phase-spec` block. Run `mdsmith check .`: the two
2608300937 records now fail MDS071 for the missing keys, proving the
rule bites. This plan's own already-migrated record stays clean, and no
other file regresses. Capture that as the red evidence — the linter is
the instrument, no Go test.

GREEN. Give the two 2608300937 records their front matter, each matching
that plan's ledger truth: `phase-1.result.md` gets `n: 1`, the ledger's
Phase 1 title, `status: "✅"`; `phase-3.result.md` gets `n: 3`, the same
title its `phase-3.md` spec carries, `status: "✅"`. Do NOT touch that
plan's `phases:` ledger — `frit doctor`/`next` still read it, and issue
111 retires it later. Confirm `mdsmith check .` is clean across every
file.

Gate: a `phase-record` missing front matter fails `mdsmith check`; the
two migrated records pass; `mdsmith check .` is clean across all plans;
`go test ./...`, `golangci-lint` and `frit doctor` stay green with no
gap for this plan.
