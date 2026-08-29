---
id: 2608280653
title: A plan may be a folder holding a fixed plan.md, beside flat plan files
status: "✅"
summary: >-
  A plan is a single `plan/<id>_<slug>.md` file today; a plan that
  needs companion files (research, fixtures, diagrams) has nowhere to
  keep them. This adds a second accepted shape,
  `plan/<id>_<slug>/plan.md`, discovered beside the flat form. The
  fleet index still keys a plan by its front-matter `id:`, so the
  contract across the fleet is unchanged; the folder name carries the
  same `<id>_<slug>` convention and doctor proves the two agree. It
  lands as one consented change: discovery, the claimed lane's name,
  the doctor scan, the docs, and the `.mdsmith.yml` widening together.
  Flat plans keep working untouched.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: Off-refs discovery reads a plan folder's plan.md as one plan
    status: "✅"
  - n: 2
    title: A claimed folder plan names its lane from the folder
    status: "✅"
  - n: 3
    title: The on-disk doctor scan sees folder plans and proves id sync
    status: "✅"
  - n: 4
    title: Proto, scaffold, skills and mdsmith teach the folder layout
    status: "✅"
---
# A plan may be a folder holding a fixed plan.md, beside flat plan files

## Goal

frit discovers a plan written as `plan/<id>_<slug>/plan.md` exactly as
it discovers `plan/<id>_<slug>.md`, so a plan can keep companion files
in its own folder. The front-matter `id:` stays the one canonical id;
the folder name follows the same `<id>_<slug>` convention and doctor
proves the folder's id prefix matches the front matter. Flat plans are
unchanged.

## Context

Today a plan is one flat markdown file. Two read paths find it, and a
third names the lane it starts.

**Off-refs discovery.** [collect.go](../internal/plans/collect.go)
lists each ref's plan tree with `TreeEntries`, which runs
`ls-tree -r` ([git.go](../internal/gitobj/git.go)) — already
recursive. So `plan/<dir>/plan.md` is *already* returned as a blob.
The gap is the opposite of missing: `markdownOnly` keeps **every**
`.md` under the tree, so a folder's companion files
(`plan/<dir>/notes.md`) are collected too and then fail
[index.Build](../internal/index/index.go) as "not a plan". Discovery
must keep flat `plan/*.md` and a folder's fixed `plan.md`, and ignore
the rest of a folder. A companion like `notes.md` is meant to be
ignored. But a dropped file named like a plan, `[0-9]*_*.md`, is a
plan in the wrong place; discovery reports it, not drops it silent.

**On-disk doctor.** [doctor.go](../internal/doctor/doctor.go) globs
`plan/[0-9]*_*.md`, one level, non-recursive. It never sees a folder
plan. It is also where an id-sync check belongs.

**Lane naming.** `defaultLanePath`
([claim.go](../cmd/frit/claim.go)) takes the slug from the plan file
stem after its `_`. For a folder plan the file is `plan.md`: stem
`plan`, no `_`, slug `plan`. The lane would be misnamed `<repo>-plan`.
The slug must come from the folder for a folder plan.

**Reuse first.** No new git subprocess kind and no second parser:
`ls-tree -r` already descends, and `planmeta.Parse` already reads the
id from front matter. A folder plan is a filter and a naming change,
not a new machinery layer — the "consumes rather than reimplements"
rule holds. The fixed inner name is one constant, `plans.FixedName =
"plan.md"`, distinct from the `PLAN.md` catalog. The folder rule —
base `plan.md`, one level deep — is one predicate; define it once so
discovery and doctor cannot drift on it. Their flat halves already
differ and stay that way: discovery keeps any `plan/*.md`, doctor globs
`[0-9]*_*.md`. Only the folder half is shared.

**Scope.** The fleet key stays `host:repo:id` with `id` from front
matter; this plan adds no second id source, only a check that the
plan's name and its front matter agree. That id-sync check covers both
shapes: a flat `<id>_` prefix and a folder `<id>_` name carry the same
latent skew, and one check catches both.

**One atomic change.** The plan globs must widen for folder plans:
the `.mdsmith.yml` repo-lint and id-uniqueness patterns, and the
`PLAN.md` catalog's own `<?catalog?>` globs, which live in PLAN.md,
not in `.mdsmith.yml`. [CLAUDE.md](../CLAUDE.md) forbids editing
`.mdsmith.yml` without explicit user consent. That consent is a
precondition, obtained before implementation starts. The whole plan
then lands as one change:
discovery, lane naming, doctor, docs and the `.mdsmith.yml` widening
together. A folder plan is never shipped half-supported — discoverable
by frit yet unlinted, unguarded for id uniqueness, or absent from the
catalog.

**Loose by design.** Discovery treats any one-level `<dir>/plan.md` as
a candidate plan, keyed by its front-matter `id:`. It does not require
the folder to be named `<id>_<slug>`. This matches flat discovery,
which already keeps any `plan/*.md` and keys it by front matter. A
stray `plan/templates/plan.md` is indexed, then flagged by the id-sync
check — exactly as a stray flat `plan/templates.md` with front matter
is today. The convention lives in the doctor and lint layer, not in
discovery. That flags a misnamed plan loudly, and keeps the two shapes
consistent.

## Tasks

1. Teach off-refs discovery to keep a folder's fixed `plan.md`, drop a
   folder's companion files, and report a dropped plan-like `.md`.
2. Name a claimed folder plan's lane from the folder, not `plan.md`.
3. Make the on-disk doctor scan see folder plans, and report any plan —
   flat or folder — whose name id disagrees with its front-matter `id:`.
4. Update `plan/proto.md`, the scaffold assets, `plan-new`, `CLAUDE.md`,
   the consented `.mdsmith.yml` globs and companion override, and the
   PLAN.md catalog globs.

## Phase 1: Off-refs discovery reads a plan folder's plan.md as one plan

`Collect` must return, per ref, the flat `plan/*.md` files and each
folder's single fixed `plan.md`, and nothing else beneath a folder. A
dropped `.md` that is named like a plan is reported, not lost. A folder
plan then reaches [index.Build](../internal/index/index.go) as exactly
one file and indexes under its front-matter id.

RED lives in `collect_test.go`
([internal/plans](../internal/plans)). Build a git tree with four
files. One is a flat `2601010000_flat.md`. One is a folder's
`2601020000_folder/plan.md`, valid, id `2601020000`. One is a companion
`2601020000_folder/notes.md`, not named like a plan. The last is a
nested plan-like `archive/2601050000_deep.md`. Assert `Collect` returns
the flat file and the folder's `plan.md` by path. Assert it does
**not** return `notes.md` or `deep.md`. Assert `deep.md` appears in the
ignored-plan list, and `notes.md` does not. A second assertion runs the
kept files through `index.Build`. Expect two entries and no "not a
plan" problem.

GREEN: add `FixedName = "plan.md"` to
[collect.go](../internal/plans/collect.go). Today `markdownOnly` keeps
every `.md` at any depth; this narrows it. Keep an entry when its path
is one segment past the dir (`<subdir>/*.md`, a flat plan), or when its
base is `FixedName` and it sits one folder deep. Measure that depth on
the subdir-relative path `markdownOnly` filters before it rejoins the
prefix, not the full repo path, so a nested `plan-dir` still counts a
folder as one deep. Drop every other `.md`. A dropped path whose base
matches `[0-9]*_*.md` is a mislaid plan: return it in an ignored list
`Collect` surfaces, which [gather.go](../internal/fleet/gather.go)
turns into a `fleet.Problem`.

Gate: the RED assertions pass; `go test ./internal/plans/...` and
`go test ./internal/index/...` are clean; `go test ./...`,
`go vet ./...` and `mdsmith check .` stay clean.

## Phase 2: A claimed folder plan names its lane from the folder

`defaultLanePath` ([claim.go](../cmd/frit/claim.go)) must build the
lane slug from the folder name for a folder plan. A claimed folder
plan then starts a lane named for its work, not for `plan.md`.

RED lives in the lane-path test next to
[claim.go](../cmd/frit/claim.go). A plan path
`plan/2601020000_folder-plans/plan.md` in a repo `acme` yields lane
path `.../acme-folder-plans`. The flat case
`plan/2601020000_folder-plans.md` still yields the same lane. Assert
both.

GREEN: when `filepath.Base(planPath) == plans.FixedName`, take the
stem from the parent folder name; otherwise keep the file-stem path.
Strip the `<id>_` prefix as today.

Gate: both RED cases pass; `go test ./cmd/frit/...` is clean;
`go test ./...`, `go vet ./...` and `mdsmith check .` stay clean.

## Phase 3: The on-disk doctor scan sees folder plans and proves id sync

`doctor.Scan` ([doctor.go](../internal/doctor/doctor.go)) must scan a
folder plan's `plan.md` for the same semantic gaps as a flat plan. It
must also add one finding for every plan, flat or folder: a plan whose
name id disagrees with its front-matter `id:`. That is the "in sync"
guarantee. A flat plan carries the same `<id>_` prefix and the same
latent skew, so the check covers both shapes, not folders alone.

RED lives in `doctor_test.go`
([internal/doctor](../internal/doctor)), against a temp `plan/` tree.
A folder `2601030000_synced/plan.md` with front-matter id `2601030000`
reports no id finding, and is scanned for goal, execution-row and tier
like a flat plan. A folder `2601040000_skewed/plan.md` whose front
matter says id `2601999999` reports the mismatch. A folder
`notanid_x/plan.md` — a non-numeric prefix over parseable front matter
— reports the mismatch too, proving the check never parses and never
crashes. A flat `2601060000_flatskew.md` whose front matter says id
`2601999999` reports the same mismatch, proving the check is not folder
-only. Front matter that does not parse makes `scanFile` skip a file
before the id check, so every fixture keeps valid front matter.

The new finding needs a name — say `id-sync` — registered where the
Check vocabulary lives: the `doctor --help` text
([main.go](../cmd/frit/main.go), pinned by its contract test) and the
`Finding.Check` comments in both
[doctor.go](../internal/doctor/doctor.go) and the report's
[doctor.go](../internal/report/doctor.go). Re-record `doctor.json` if
the fixture grows a case.

GREEN: extend path discovery to see a folder's `plan.md` one level
deep, matching the set discovery keeps: base `plan.md`, one folder
deep. Then doctor and discovery agree on what a folder plan is. Compare
names as strings, so the check is total and never parses. Take the
plan's leading id token — the folder name, or the flat file stem —
before its first `_`. Render the front-matter `id:` as decimal. Emit
`id-sync` when the two differ. A non-numeric prefix (`notanid_x`), a
name with no `_`, and a numeric prefix that is not the id all differ,
so each is a mismatch — never a crash, never a skip. Sort remains by id
then check. Folder-plan schema comes online when Phase 4 widens the
`plan`-kind glob; the two land together, and Phase 4's gate asserts it.

Gate: build frit; the RED cases pass; `go test ./internal/doctor/...`
is clean; `go run ./cmd/frit doctor` still runs green over this repo's
own flat plans with no new finding; `go test ./...`, `go vet ./...`,
`golangci-lint run` and `mdsmith check .` stay clean.

## Phase 4: Proto, scaffold, skills and mdsmith teach the folder layout

The shipped schema, the scaffold, the agent-facing docs, and the
`.mdsmith.yml` globs must all describe the folder option, or an agent
never learns it and a folder plan is only half-supported. Consent to
the `.mdsmith.yml` change is obtained before this phase; it lands here
with the rest.

Changes:

- [proto.md](../internal/scaffold/assets/proto.md) **and the repo's
  own** [plan/proto.md](../plan/proto.md): broaden the `filename`
  require rule to accept the folder shape's `plan.md`, and document the
  folder layout in the conventions comment. `TestShippedProtoMatchesRepo`
  pins the two byte-equal, so edit both or the pin fails.
- The `.mdsmith.yml` plan globs and the scaffold's
  [mdsmith.yml](../internal/scaffold/assets/mdsmith.yml): add the folder
  shape (`plan/*/plan.md`) in three places. The `plan` kind's
  `path-pattern` gates doctor's schema finding, so add it there. Add it
  to the `plan` `kind-assignment` glob too. Add it to
  `unique-frontmatter` `include`, so two folder plans cannot share an id
  the fleet key relies on.
- `.mdsmith.yml` **overrides**: a folder plan holds freeform companions
  (research, fixtures, diagrams), which trip the default readability,
  structure and strict cross-reference rules. Add an `overrides` entry
  for a plan folder's companion `.md` — every `plan/*/*.md` but
  `plan.md` — that steps those rules aside, the same override
  [docs/research](../docs/research) already carries. `plan.md` keeps the
  `plan` kind and its full checks; scope the override to exclude it.
- [PLAN.md](../PLAN.md): widen the `glob:` list in both `<?catalog?>`
  directives to add `plan/*/plan.md`. Those source globs live in
  PLAN.md, not `.mdsmith.yml`; the mdsmith merge-driver regenerates the
  catalog from them, so a folder plan the glob misses is silently
  dropped from PLAN.md and any hand-added row is deleted on the next
  merge.
- [CLAUDE.md](../CLAUDE.md): note the two accepted plan shapes where
  the layout is described.
- [plan-new](../.claude/skills/plan-new/SKILL.md) and its canonical
  asset: one line offering the folder form for a plan with companions.
- Note in `plan-new` and the conventions comment that a folder plan's
  Markdown links to repo paths take one extra `../`: it sits a level
  deeper than a flat plan, so `cross-file-reference-integrity` breaks on
  a copied `../` path.
- Regenerate frit's own `.claude/skills` with
  `go run ./cmd/frit skills --via "go run ./cmd/frit"`.

Gate: this phase ships skills, a schema claim, and the glob change, so
it gates on the built frit. Scaffold a throwaway repo with
`go run ./cmd/frit init`. Drop a folder plan into its `plan/`. Confirm
`go run ./cmd/frit plans` lists it and `go run ./cmd/frit doctor`
reports it clean. Confirm `mdsmith check .` now lints the folder plan,
that a freeform companion `.md` beside it passes, and that a duplicate
id is caught. `TestDogfoodCopiesMatchCanonical` stays clean.

## Execution

Phase 1 is the proving slice: it fixes the discovery filter and the
fixture shape (a built git tree with a folder plan) that Phases 2 and 3
copy. Phase 2 is a small naming change. Phase 3 adds a scan path and a
check. Phase 4 lands the docs, schema and consented globs, gated by
running the built frit.

| Phase                     | Design | Implement | Gate that catches a wrong answer                                                    |
| ------------------------- | ------ | --------- | ----------------------------------------------------------------------------------- |
| 1 Off-refs discovery      | opus   | sonnet    | Collect keeps a folder's plan.md, drops companions, reports a mislaid plan          |
| 2 Lane names from folder  | opus   | sonnet    | a folder plan's lane path is `<repo>-<slug>`, not `<repo>-plan`                     |
| 3 Doctor scan and id sync | opus   | sonnet    | doctor.Scan reports a flat or folder plan whose name id disagrees with front matter |
| 4 Docs, schema, globs     | opus   | sonnet    | built frit lists, doctors and lints a real folder plan; a dup id is caught          |

## Acceptance Criteria

- [x] `Collect` returns a folder's `plan.md`, drops its companion
      files, reports a dropped plan-like `.md` as a problem, and
      `index.Build` indexes the folder plan under its front-matter id
- [x] A claimed folder plan's lane path takes its slug from the folder,
      not from `plan.md`
- [x] `doctor.Scan` scans a folder plan for the goal, execution-row and
      tier gaps, and reports any plan — flat or folder — whose name id
      disagrees with its front-matter `id:`
- [x] `plan/proto.md`, the scaffold assets, `plan-new`, `CLAUDE.md` and
      the consented `.mdsmith.yml` globs describe the folder layout, and
      a folder plan lints and is id-guarded
- [x] Both `<?catalog?>` globs in PLAN.md include the folder shape, so
      a folder plan appears in the regenerated catalog
- [x] A freeform companion `.md` in a plan folder passes
      `mdsmith check .` under the new override
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
