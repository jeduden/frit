---
n: 1
title: Kinds carry summary and discriminator; the Phases catalog interleaves
status: "✅"
result: true
summary: The interleaved Phases catalog renders a spec row directly followed by its result's summary row against a fixture; a stale body trips MDS019.
---
## Handoff

The interleaved `## Phases` catalog renders end to end against an
in-memory fixture: a spec row for `phase-1.md`, directly followed by
its `phase-1.result.md`'s `↳` summary row. A body missing the summary
row trips MDS019.

**Discriminator.** `result` (bool): `false` on every `phase-N.md`,
`true` on every `phase-N.result.md`. Landed as `result?: 'bool'` on
both `phase-spec` and `phase-record` in [.mdsmith.yml](../../.mdsmith.yml)
— the `?` keeps it optional so every existing phase file still lints
without it. `phase-record` also carries `summary?: 'string & != ""'`.
Phase 2 backfills every real file with both keys, then tightens both
to required (dropping the `?`).

**The row-expr.** On the catalog directive:
`[if result {"|  | ↳ | \(summary) |"}, if !result {"| \(n) | \(status)
| [\(title)](phase-\(n).md) |"}][0]`, with `sort: numeric:n` — the
path tie-break (`phase-1.md` sorts before `phase-1.result.md`) keeps
each summary row directly beneath its spec, no explicit path handling
needed.

**Render-test gotcha for Phase 2/3.** `mdsmith.Session` (used by
`phaseKindsSession`/`catalogFixtureSession` in
[internal/planmeta/kinds_test.go](../../internal/planmeta/kinds_test.go))
lints via `RunSource`, which wires `lint.File.FS` to the *whole*
`MemWorkspace` root rather than the on-disk CLI's per-file-directory
`os.Root` scoping. A catalog directive's `glob:` pattern in a
`MemWorkspace` fixture must therefore be workspace-relative (e.g.
`"plan/2601010000_x/phase-*.md"`), not the file-relative form
(`"phase-*.md"`) real, on-disk folder-plan catalogs use — those still
lint correctly under `mdsmith check .` because the CLI's disk-backed
`File.FS` *is* scoped per directory. Reuse `catalogFixtureSession` and
this glob-pattern shape for Phase 2's dogfood-match/scaffold catalog
tests and Phase 3's doctor fixtures; do not copy the file-relative glob
form into a `MemWorkspace` test.

**Not touched this phase.** No real `phase-N.md`/`phase-N.result.md`
carries `result`/`summary` yet, and the live `## Phases` directives
(e.g. plan 2608310418) still render spec-only — both are Phase 2.
`plan/proto.md` and its scaffold twin now describe the interleaved
convention in prose only.

`mdsmith check .`, `go test ./...`, and
`golangci-lint run ./...` are clean.
