---
n: 3
title: plan-new writes the phase front matter and the generated catalog
status: "🔲"
---
Close the loop. Phases 1 and 2 made the `phase-spec` and `phase-record`
kinds require `{n, title, status}`, so any plan `plan-new` authors
without it now fails the linter. Teach the skill to write the front
matter and the generated `## Phases` catalog, so a new folder plan lints
on the first try and gets the derived status table for free.

Assumes: the [plan-new skill](../../internal/skills/assets/plan-new/SKILL.md)
already defaults to a folder plan with each phase in its own
`phase-N.md` and already drops the `phases:` ledger for new plans (its
step 4 says so). It does not yet mention the required front matter or
the catalog. The skill ships in two copies — the canonical asset and
the dogfood [copy](../../.claude/skills/plan-new/SKILL.md) — pinned
equal by `TestDogfoodCopiesMatchCanonical`, which substitutes the
`{{frit}}` token for `go run ./cmd/frit`. Only the scaffold carries a
`proto.md`; `internal/skills/assets` carries none, so nothing there to
mirror.

RED. Add a Go test in
[internal/skills](../../internal/skills/skills_test.go) asserting the
canonical `plan-new` asset instructs the phase-file front matter — the
`{n, title, status}` fields — and the generated `## Phases` catalog. It
fails at HEAD, where the skill says neither.

GREEN. Edit the canonical asset. In the phase-file step, say each
`phase-N.md` opens with `{n, title, status}` front matter. Add that the
folder `plan.md` carries a `## Phases` catalog, regenerated from those
files. Keep the "no `phases:` ledger" line: existing plans keep theirs,
and only new plans skip it. Stay within the skill kind's caps — file
length, section length, and the 650-token budget. Then regenerate the
dogfood copy with the built binary:
`go run ./cmd/frit skills --force --via "go run ./cmd/frit" .`.

Gate: the new test passes. `TestDogfoodCopiesMatchCanonical` stays
green. The built `frit skills` writes a dogfood `plan-new` copy matching
the canonical asset. `go test ./...`, `golangci-lint`, `mdsmith check .`
and `frit doctor` stay clean, with no gap for this plan. The skill still
mentions `{{frit}} doctor` and `phase-1.md`, and names no `headroom`.
