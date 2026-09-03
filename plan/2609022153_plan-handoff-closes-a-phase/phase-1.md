---
n: 1
title: The plan-handoff skill in the bundle
status: "✅"
result: false
---
Add a `plan-handoff` skill to the bundle and regenerate frit's own
dogfood copy. The skill closes one phase. It records the handoff in
the shape the plan uses. A single-file plan gets a `## Handoff`
heading inline in its `plan.md`. A directory plan gets a separate
`phase-N.result.md` beside it, carrying the literal `## Handoff`
heading frit's resume reads. Either way it flips the phase's status
inside the commit that lands the work, then ends with a safe-to-clear
cue: the handoff is durable, so the session may be cleared and the
next phase run fresh.

**Assumes.** The bundle's canonical text lives under
[internal/skills/assets](../../internal/skills), one `SKILL.md` per
skill directory. `frit skills` lays it down with `{{frit}}` filled by
`--via`. frit's own `.claude/skills` is the regenerated dogfood
output, pinned by `TestDogfoodCopiesMatchCanonical` in
[internal/skills/skills_test.go](../../internal/skills/skills_test.go).
That test fails if a dogfood copy diverges from its canonical source.
The `skill` kind in [.mdsmith.yml](../../.mdsmith.yml) caps each skill
at 650 tokens and bans filler prose. The bundle's skill list is named
in [internal/skills/skills.go](../../internal/skills/skills.go) and
described in [docs/development.md](../../docs/development.md). The two
plan shapes are the ones `plans.IsFolderPlanFile` already tells apart:
a single file `plan/<id>_<slug>.md`, and a directory
`plan/<id>_<slug>/plan.md` with its `phase-N.md` files beside it.

**RED.** A test that fails until the new skill is in the bundle. If
`TestDogfoodCopiesMatchCanonical` and the bundle-count assertions in
[internal/skills/skills_test.go](../../internal/skills/skills_test.go)
key off the set of skills, extend the expected set to include
`plan-handoff` first — the suite then fails until the canonical asset
and the regenerated dogfood copy both exist. Reuse the existing
per-skill assertions rather than adding fresh scaffolding.

**GREEN.** Write `internal/skills/assets/plan-handoff/SKILL.md`:

- Front matter `name: plan-handoff` and a `description` whose triggers
  are closing a phase — "hand off phase N", "close the phase", "write
  the handoff", "wrap up before a new session".
- A `## Method` that splits on the plan's shape. A single-file plan:
  write or replace a `## Handoff` heading in its `plan.md` stating the
  outcome and what the next phase inherits, and flip the phase's
  `phases:` entry to ✅. A directory plan: write `phase-N.result.md`
  with `{n, title, status: "✅", result: true, summary}` and a `##
  Handoff` section, and flip the matching `phase-N.md` `status:`.
- Both shapes: commit the close riding the work; and, when it is the
  last phase, tick Acceptance Criteria and move the plan `status:` to ✅
  with `mdsmith fix PLAN.md`.
- A closing step: the handoff is now durable in the repo, so this
  session's working context is disposable — clear it and start the next
  phase in a fresh session, which reads only the bundle `frit phase`
  assembles.

Add `plan-handoff` to the skill list in
[internal/skills/skills.go](../../internal/skills/skills.go) and its
one-line description in [docs/development.md](../../docs/development.md).
Regenerate the dogfood copy: `go run ./cmd/frit skills --via "go run
./cmd/frit" --force` into frit's own `.claude/skills`.

**Gate.** A skill phase gates on the claim, not the copy. Build `frit`,
run `frit skills --via "go run ./cmd/frit"` into a scratch repo, and
confirm `plan-handoff/SKILL.md` is laid down with `{{frit}}`
substituted. Then `mdsmith check .` is clean — the new skill meets the
650-token budget and the readability caps — and the full `go test
./...` and lint pass, `TestDogfoodCopiesMatchCanonical` among them.
