---
name: plan-new
description: >-
  Create a plan file under plan/ that passes mdsmith on the first try:
  minute-precision id, phases frontmatter, extractable "## Phase N"
  sections, an ## Execution tier table, and a refreshed PLAN.md.
  Trigger on "plan this work", "create a plan", "new plan for X", or
  "capture this as a plan".
---
# plan-new

Create one plan file under `plan/`, conforming to `plan/proto.md`.
frit reads these files; it does not write them, so this skill is
mdsmith and an editor, not a frit verb.

## Method

1. **Id.** `date -u +%y%m%d%H%M`. If `plan/<id>_*` exists, add a
   minute and retry. The id is both the frontmatter `id:` and the
   filename prefix: `plan/<id>_<kebab-slug>.md`.
2. **Reuse first.** Search the codebase for machinery that already
   does what the plan needs. `## Context` must name what was searched
   and why each candidate was or was not reused.
3. **Phase 1 is a proving slice** — a minimal end-to-end slice that
   demonstrates the Goal and fixes the test approach every later phase
   copies. Declare Phase 1 concretely, plus only the later phases that
   are already certain; append the rest once the slice shows the real
   shape. In `## Tasks`, write `2. (determined after Phase 1)`.
4. **Write the file** to the `plan/proto.md` shape: frontmatter (`id`,
   `title`, `status: "🔲"`, `summary`, `model`, `depends-on`), then
   `## Goal`, `## Context`, `## Tasks`, one self-contained
   `## Phase N: <title>` per phase (RED spec, GREEN sites, gate), an
   `## Execution` table, and `## Acceptance Criteria`. A phase section
   must be executable without reading the others.
5. **Tier per phase** in the Execution table: the cheapest tier a loud
   gate makes safe. Design stays opus; implementing from a written
   assertion is cheap. Set frontmatter `model:` to the dominant
   implement tier.
6. **Lint and index.** `mdsmith check plan/<file>` (fix line length 80
   and long sentences), `mdsmith fix PLAN.md`, then `mdsmith check .`.
7. **Health-check.** `go run ./cmd/frit doctor` scans every plan on disk, so
   check its output for this plan's id: no missing Goal, no Execution row
   short of a phase, no tier `plan/proto.md` rejects. Fix the plan,
   not the check.
8. **Commit** the plan file and PLAN.md together: `plan <id>: <title>`.

## Notes

- A single-phase trivial plan may omit `phases:` and phase sections;
  `## Tasks` alone carries them.
- Never renumber phases later — append. `depends-on:` lists plan ids
  this plan cannot start before.
