---
n: 1
title: A phase file carries its status and a generated table shows it
status: "✅"
---
## Handoff

The `phase-spec` kind now requires `{n, title, status}` front matter
(MDS071). RED proved it: a throwaway phase spec with no front matter
passed `mdsmith check` at HEAD and failed once the rule landed. Two
existing phase-spec files gained front matter to keep the suite green —
this plan's own `phase-1.md`, and the one pre-existing folder plan's
`phase-3.md` (n 3, ✅, matching its finished ledger). This plan's
`plan.md` now carries a live `## Phases` catalog that mdsmith
regenerates from `phase-*.md` front matter, sorted numerically by `n`,
one row per phase linking `phase-N.md`.

The seam, solved. `plan/proto.md` is the plan schema, and its legacy
parser knows only required-literal headings or the `## ...` wildcard —
it cannot mark a single named heading optional. A literal `## Phases`
in proto forces the section onto every plan (verified: an existing flat
plan then failed "required by schema"). So `## Phases` is NOT a proto
literal; the existing `## ...` slot between Tasks and Acceptance already
admits it optionally (the same slot that admits Context and Execution).
proto's conventions comment documents the optional catalog and points
at this plan as the live example. A second trap: bare `{field}` braces
in the proto body are read as MDS020 sync placeholders against every
plan, so the comment describes the row in words, not `{n}`/`{title}`.

Phase 2 inherits: apply the same `required-frontmatter` to the
`phase-record` kind, then migrate every result file that lacks it —
2608300937's `phase-1.result.md` and `phase-3.result.md`, plus this
plan's own result files (this one already carries front matter). Keep
this plan's `phases:` ledger flipping in step with the phase-file
status until issue 111 retires the ledgers.
