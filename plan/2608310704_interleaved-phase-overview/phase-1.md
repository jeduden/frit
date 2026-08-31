---
n: 1
title: Kinds carry summary and discriminator; the Phases catalog interleaves
status: "🔲"
---
Prove the interleaved overview end-to-end on the smallest slice. Teach
the phase kinds the two new fields. Prove a folder plan's `## Phases`
catalog renders a spec row followed by its result-summary row. Fix the
render-test approach the later phases reuse.

**Assumes.** The `phase-spec` and `phase-record` kinds already type `n`
as an integer or a split-phase token like `3b`, with `title` and
`status` declared in a closed `frontmatter:` map (landed groundwork).
mdsmith's row template runs on cuelite, whose
only builtins are `strings.Join` and `len` — so the catalog cannot tell
a spec file from a result file by name and must branch on a front-matter
field both carry.

**Value.** A reader of `plan.md` sees each phase's spec and, directly
beneath it, how that phase turned out — the plan's full state in one
table — without opening the result files.

**Keep every commit green.** The two new fields land as *optional* typed
fields this phase, so every existing phase file still lints and
`mdsmith check .` stays clean. Phase 2 backfills the real files and the
live catalogs and only then tightens the fields to required. The
`## Phases` directive that live plans carry is not changed here; the
interleaved directive is proven against an in-memory fixture and written
into the `plan-new` skill in Phase 2.

**RED.** Add a test beside the existing kind tests in
[internal/planmeta/kinds_test.go](../../internal/planmeta/kinds_test.go).
Use the `phaseKindsSession` harness that lints in-memory files against
the real `.mdsmith.yml`. Build a fixture folder plan under a synthetic
`plan/<id>_x/` path. Its `plan.md` carries a `## Phases` catalog that
globs both `phase-*.md` and `phase-*.result.md` and sorts `numeric:n`.
The catalog renders via a `row-expr` branching on the boolean
discriminator: a spec row for a phase file, an indented `↳ summary` row
for a result file. Add a `phase-1.md` and a `phase-1.result.md` carrying
the discriminator, and a `summary` on the result. Assert the session
regenerates the body to the interleaved table — spec row then
result-summary row — and that a body missing the result row trips
MDS019. At HEAD the `summary` and the discriminator are undeclared keys
the closed `frontmatter:` maps reject, so the fixture does not render.

**GREEN.** In [.mdsmith.yml](../../.mdsmith.yml): add an optional boolean
discriminator to both the `phase-spec` and `phase-record` `frontmatter:`
maps, and an optional non-empty `summary` string to `phase-record`. Pick
the discriminator name in the spec below. The fixture now renders the
interleaved table and the stale-body case trips MDS019; the render test
is green.

**Also update the conventions prose.**
[plan/proto.md](../proto.md) and its scaffold twin
[internal/scaffold/assets/proto.md](../../internal/scaffold/assets/proto.md)
describe the `## Phases` catalog as spec-only in their conventions
comment. Update that prose to describe the interleaved table, the result
`summary` line, and the discriminator, so the convention matches what
Phase 2 rolls out. This is prose only — no live directive lives in
proto.md — so `mdsmith check .` stays green.

**Discriminator name.** Use a boolean that reads honestly on both files.
`result: true` on a `phase-N.result.md`, `result: false` on a
`phase-N.md`, so the row-expr branches `[if result {…}, if !result {…}][0]`.
Record the chosen name in the Handoff so Phase 2 backfills the same key.

**Gate.** A fixture folder plan regenerates a spec row immediately
followed by its result-summary row, and a body missing the result row
trips MDS019; `mdsmith check .` and `go test ./...` are green.
