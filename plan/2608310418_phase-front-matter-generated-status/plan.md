---
id: 2608310418
title: Phase front matter gives per-phase status one generated home
status: "🔳"
summary: >-
  A folder plan carries each phase as a freeform phase-N.md with no
  front matter, so per-phase status has to live in the plan.md phases:
  ledger, hand-maintained and free to drift from the work. Give each
  phase file {n, title, status} front matter and render a generated
  Phases catalog over a relative phase-*.md glob, so a phase closes
  by flipping its own file and the table regenerates. plan-new writes
  the front matter and the catalog and drops the ledger for new plans.
  Existing ledgered plans keep working untouched: frit doctor and next
  still read the ledger until issue 111 teaches them the front matter,
  so this change does not remove any ledger frit still reads.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: A phase file carries its status and a generated table shows it
    status: "✅"
  - n: 2
    title: A phase record requires the front matter and the existing records migrate
    status: "✅"
  - n: 3
    title: plan-new writes the phase front matter and the generated catalog
    status: "🔲"
---
# Phase front matter gives per-phase status one generated home

## Goal

Give a folder plan's phase files `{n, title, status}` front matter and
a generated `## Phases` catalog. Per-phase status then has one home —
the phase file — surfaced by a derived table that a hand-flipped ledger
cannot drift from. This is issue 110.

## Context

A folder plan keeps each phase as a freeform `phase-N.md` spec with no
front matter, beside its `plan.md`. Per-phase state lives in the
`plan.md` front-matter `phases:` ledger, hand-maintained. An
`<?catalog?>` directive reads front matter, not body prose, and the
phase files expose none. So the status cannot be a generated table, and
the ledger can drift from the work.

**Reuse first.** The `<?catalog?>` directive already renders the plan
index in [PLAN.md](../../PLAN.md) over a glob with `header`, `row` and
`sort`. The same machinery renders a per-plan `## Phases` table over a
relative `phase-*.md` glob, so nothing new is invented. The phase kinds
live in [.mdsmith.yml](../../.mdsmith.yml) as `phase-spec` and
`phase-record`, each pinning only the filename today. Adding
`required-frontmatter: [n, title, status]` there makes the front matter
load-bearing. The plan template is [plan/proto.md](../proto.md),
mirrored byte-for-byte by the scaffold copy
[internal/scaffold/assets/proto.md](../../internal/scaffold/assets/proto.md);
a Go test pins them equal. `plan-new` is a skill in two copies: the
canonical [SKILL.md](../../internal/skills/assets/plan-new/SKILL.md) and
the dogfood [copy](../../.claude/skills/plan-new/SKILL.md) `frit skills`
writes.

**The migration constraint.** Making front matter *required* on the
phase kinds breaks any phase file that lacks it. So `mdsmith check .`
stays honest only if two sets of files gain front matter in the same
change: the one existing folder plan
([2608300937](../2608300937_per-phase-files-token-cheap-resume/plan.md),
its `phase-3.md`), and this plan's own phase files.

**The ordering constraint — why the ledger stays for now.** `frit
doctor` and `frit next` read per-phase state from the `phases:` ledger,
and from the result-file `## Handoff`. Removing a ledger a live verb
reads would blind it. So this plan only makes `plan-new` stop writing
the ledger for *new* plans, and leaves every existing ledger in place.
The companion issue 111 teaches doctor and next to read the front
matter; the old ledgers can retire after it lands. The `## Phases`
catalog section must be *optional* in the plan structure, so existing
flat and ledgered plans without it still pass `mdsmith check .`.

## Tasks

1. Phase 1 (proving slice): `phase-spec` requires `{n, title, status}`
   front matter; a folder plan renders a generated `## Phases` catalog
   over a relative `phase-*.md` glob; this plan's own phase files carry
   the front matter; `mdsmith check .` stays clean across every
   existing plan.
2. Phase 2: `phase-record` requires the same front matter; migrate the
   existing folder plan's phase files to carry it; the catalog reflects
   their status.
3. Phase 3: `plan-new` writes phase front matter and the `## Phases`
   catalog and stops writing the `phases:` ledger for new plans; both
   skill copies and the scaffold `proto.md` mirror move together; the
   `frit skills` claim gate confirms the dogfood copy.
4. Later phases as handoffs reveal them.

## Execution

| Phase | Title                                                          | Tier   | Gate                                                                                                                                                               |
| ----- | -------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | A phase file carries its status and a generated table shows it | sonnet | `mdsmith` renders the `## Phases` table from phase front matter; a phase-spec missing front matter fails `mdsmith check`; `mdsmith check .` clean across all plans |

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

| #   | Status | Phase                                                                                   |
| --- | ------ | --------------------------------------------------------------------------------------- |
| 1   | ✅     | [A phase file carries its status and a generated table shows it](phase-1.md)            |
| 2   | ✅     | [A phase record requires the front matter and the existing records migrate](phase-2.md) |
| 3   | 🔲     | [plan-new writes the phase front matter and the generated catalog](phase-3.md)          |
<?/catalog?>

## Acceptance Criteria

- [ ] `phase-spec` and `phase-record` kinds require `{n, title, status}`
      front matter
- [ ] A folder plan renders a generated `## Phases` table over a
      relative `phase-*.md` glob; a phase closes by flipping its own
      file's `status` and the table regenerates
- [ ] `plan-new` writes phase front matter and the catalog and drops
      the `phases:` ledger for new plans; both skill copies agree and
      the `frit skills` claim gate passes
- [ ] Every existing ledgered plan still passes `mdsmith check .`; no
      ledger that `frit doctor`/`next` reads is removed by this plan
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
