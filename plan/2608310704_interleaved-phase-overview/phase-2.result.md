---
n: 2
title: Every phase file and live plan.md adopts the front matter and catalog
status: "✅"
result: true
summary: Every real phase file carries result/summary, both are now required, and every live Phases catalog renders interleaved.
---
## Handoff

`result` and `summary` are now required front matter: `.mdsmith.yml`
dropped the `?` on both and added them to each kind's
`required-frontmatter.fields`. RED proved it — a `phase-1.md` missing
`result`, and a `phase-1.result.md` missing `result`/`summary`, each
trip MDS020/MDS071 once the fields are required, where they lint clean
at HEAD. The four pre-existing "Accepts…PhaseNumber" fixtures in
[kinds_test.go](../../internal/planmeta/kinds_test.go) needed
`result`/`summary` added too, since they now trip the new requirement
as unrelated noise on top of the phase-number case they actually test.

Every real `phase-N.md` carries `result: false`; every real
`phase-N.result.md` carries `result: true` plus a `summary` line drawn
from its own Handoff — 8 specs, 8 records (this plan's own phase-2
files included). The three live `## Phases` catalogs — plan 2608310418,
2608310454, and this plan's own — now glob both `phase-*.md` and
`phase-*.result.md` and use Phase 1's row-expr, interleaved end to end.
This plan's own table, mid-phase, proved the "open phase, no result
yet" case: phase 2's spec row rendered alone until this Handoff exists.

`internal/skills/assets/plan-new/SKILL.md` documents both fields and
the interleaved catalog; `go run ./cmd/frit skills --force --via "go
run ./cmd/frit" .` regenerated the `.claude/skills` dogfood copy —
`TestDogfoodCopiesMatchCanonical` is the gate. `internal/scaffold`'s
starter `mdsmith.yml` is independent content, not pinned to the repo's
own config, so it was left untouched.

`mdsmith check .`, `go test ./...`, `golangci-lint run ./...`, and
`frit doctor` are all clean.
