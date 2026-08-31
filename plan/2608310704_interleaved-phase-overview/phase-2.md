---
n: 2
title: Every phase file and live plan.md adopts the front matter and catalog
status: "✅"
result: false
---
Phase 1 landed `result` and `summary` as optional fields. Backfill
every real phase file with them, tighten both to required, regenerate
every live folder plan's `## Phases` catalog into the interleaved
form, and teach `plan-new` to write both from here on.

**RED.** In
[internal/planmeta/kinds_test.go](../../internal/planmeta/kinds_test.go),
add a case beside the Phase 1 fixtures: a `phase-1.md` with no `result`
key, and a `phase-1.result.md` with `result` but no `summary`. At HEAD
(`result?`/`summary?` optional) both lint clean; assert that instead —
`hasDiagnostic(diags, "required-structure")` true for each. Both fail
today.

**GREEN, schema.** In [.mdsmith.yml](../../.mdsmith.yml): drop the `?`
on `result` (`phase-spec`, `phase-record`) and `summary`
(`phase-record`), and add `result` to `phase-spec`'s
`required-frontmatter.fields`, `result` and `summary` to
`phase-record`'s. The new RED cases go green; Phase 1's fixtures (which
already carry both) are unaffected.

**GREEN, backfill.** Every `plan/*/phase-*.md` gets `result: false`.
Every `plan/*/phase-*.result.md` gets `result: true` plus a `summary:`
one-liner drawn from its own `## Handoff` — seven specs, eight records,
all already closed (✅). `mdsmith check .` names any file this misses.

**GREEN, live catalogs.** Three `plan/*/plan.md` files carry a
`## Phases` `<?catalog?>`: 2608310418, 2608310454, and this plan's own.
Rewrite each directive to Phase 1's proven shape — `glob:` both
`phase-*.md` and `phase-*.result.md` (on-disk, so the file-relative
form works, unlike the `MemWorkspace` fixture — see Phase 1's Handoff),
`sort: numeric:n`, and the `row-expr`:
`[if result {"|  | ↳ | \(summary) |"}, if !result {"| \(n) | \(status)
| [\(title)](phase-\(n).md) |"}][0]`. `mdsmith fix` each file to
regenerate its body.

**GREEN, plan-new.** Edit the canonical
[plan-new's SKILL.md](../../internal/skills/assets/plan-new/SKILL.md),
never the dogfood copy directly. Step 3's `phase-1.md` front matter
becomes `{n, title, status, result: false}`. Step 4's catalog
description switches from the spec-only `row:` template to the
interleaved `row-expr` above. Note that closing a phase also adds
`result: true` and `summary` to its `phase-N.result.md`. Run
`go run ./cmd/frit skills .` to regenerate the dogfood copy at
`.claude/skills/plan-new/SKILL.md` — `TestDogfoodCopiesMatchCanonical`
is the gate, not a hand-edit.

**Not this phase.** `frit doctor`'s filename-vs-`n` check is Phase 3;
leave `internal/doctor` untouched.

**Gate.** `mdsmith check .` clean after the backfill and every catalog
regenerated; `go test ./...` green, including
`TestDogfoodCopiesMatchCanonical` and the scaffold tests; `go tool
-modfile=tools/go.mod golangci-lint run` clean.
