---
id: 2608310704
title: Interleaved phase overview with result summaries
status: "🔳"
summary: >-
  A folder plan's `## Phases` catalog lists only its phase specs, so
  plan.md shows what each phase is but not how it turned out. Fold the
  result files into the same overview: each phase's spec row, then its
  result-summary row directly beneath, so plan.md reads as the plan's
  full state. That needs a `summary` line on every result file and a
  discriminator the catalog's cuelite row template can branch on, since
  cuelite cannot tell a spec file from a result file by name. Separately,
  frit trusts the filename for a phase's number while the catalog renders
  from front-matter `n`; a doctor check ties the two so they cannot drift.
model: sonnet
depends-on: [2608310454]
---
# Interleaved phase overview with result summaries

## Goal

A folder plan's `## Phases` catalog becomes one interleaved table over
both phase specs and their result files. Each phase's spec row is
followed by its result-summary row, so `plan.md` shows the plan's full
state. And `frit doctor` flags any phase whose front-matter `n`
disagrees with its filename.

## Context

Groundwork already landed in the working tree. The `phase-spec` and
`phase-record` kinds in [.mdsmith.yml](../../.mdsmith.yml) now type `n`
as an integer or a split-phase token like `3b` (`int | =~"^[0-9]+[A-Za-z]+$"`),
so a split phase still lints. Both declare `title` and `status`
alongside it, because mdsmith wraps a kind's `frontmatter:` map in
`close({...})`. RED/GREEN tests pin it, in
[internal/planmeta/kinds_test.go](../../internal/planmeta/kinds_test.go).

**The overview reads only specs today.** A folder plan's `## Phases`
catalog globs `phase-*.md` and excludes `phase-*.result.md`
(see [plan/proto.md](../proto.md) and any live folder plan), so it shows
each phase's title and status but nothing about how the phase turned
out. The result file already holds that record; it is simply left out of
the table.

**Why a discriminator field is needed.** mdsmith's row template runs on
cuelite, an in-house CUE subset whose only builtins are `strings.Join`
and `len`; `strings.HasSuffix` is unsupported and `len` of a struct is
rejected. So a single catalog cannot decide spec-versus-result from the
filename inside the row expression. The branch must read a front-matter
field present on every phase file — a boolean the spec and result files
both carry, distinguishing the two. A `row-expr` then renders a spec row
or an indented result-summary row per file, and `sort: numeric:n` with
its path tie-break keeps each result directly beneath its spec.

**Why doctor owns the filename↔n check.** frit derives a folder plan's
phase number from the filename — `specFileRE` in
[internal/planmeta/resume.go](../../internal/planmeta/resume.go) — while
the catalog renders from front-matter `n`. mdsmith cannot bind them: its
`filename` schema is a plain `filepath.Match` glob with no front-matter
interpolation, and a catalog-generated link to a `phase-{n}.md` that
does not exist is filtered out of MDS027's link check. So the two can
drift silently. doctor is the enforcement point.

**Reuse first.** `checkIDSync` in
[internal/doctor/doctor.go](../../internal/doctor/doctor.go) already
compares a plan's front-matter `id` against its on-disk name token and
reports a mismatch. That is the exact shape the phase check copies: one
finding per divergent phase. `doctor.Scan` already reads a folder plan's
phases from disk via `planmeta.PhasesFromDir`, which globs the phase
files. So the filename number and the front-matter `n` are both in hand.
The `phaseKindsSession` harness in `kinds_test.go` already lints
in-memory fixtures against the real `.mdsmith.yml`. The catalog-render
and schema phases test the same way.

## Tasks

1. Phase 1 (proving slice): the `phase-record` kind carries a required
   non-empty `summary`, both phase kinds carry the boolean discriminator,
   and the `## Phases` catalog in [plan/proto.md](../proto.md) renders an
   interleaved spec-then-result table via a cuelite `row-expr`. A fixture
   folder plan regenerates the interleaved table and a stale body is
   caught.
2. Phase 2: backfill every existing phase file with the discriminator
   and every result file with a `summary`, regenerate every live
   folder-plan `## Phases` catalog, and update the `plan-new` skill (both
   copies) and proto conventions to document the new front matter and
   catalog — `mdsmith check .` and the dogfood-match and scaffold tests
   stay green.
3. Phase 3: `frit doctor` flags a folder plan's phase whose front-matter
   `n` differs from its filename number, mirroring the `id-sync` check.

## Execution

| Phase | Title                                                                 | Tier   | Gate                                                                                                                              |
| ----- | --------------------------------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Kinds carry summary and discriminator; the Phases catalog interleaves | opus   | A fixture folder plan regenerates a spec row then its result-summary row; a stale body trips MDS019; `go test ./...` green        |
| 2     | Every phase file and live plan.md adopts the front matter and catalog | sonnet | `mdsmith check .` green after backfilling all phase files and regenerating every catalog; dogfood-match and scaffold tests green  |
| 3     | doctor flags a phase whose front-matter n differs from its filename   | sonnet | A folder-plan fixture with a `phase-2.md` carrying `n: 5` is unflagged by doctor at HEAD and flagged after; `go test ./...` green |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                               |
| --- | ------ | ----------------------------------------------------------------------------------- |
| 1   | ✅     | [Kinds carry summary and discriminator; the Phases catalog interleaves](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] A folder plan's `## Phases` catalog renders each phase's spec row
      followed by its result-summary row, and an open phase with no
      result file shows a spec row alone
- [ ] Every result file carries a non-empty `summary`; every phase file
      carries the discriminator; `mdsmith check .` is clean
- [ ] `frit doctor` flags a phase whose front-matter `n` differs from its
      filename number, and reports nothing when they agree
- [ ] `plan-new` and `plan/proto.md` document the new front matter and
      the interleaved catalog; the dogfood-match and scaffold tests pass
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
