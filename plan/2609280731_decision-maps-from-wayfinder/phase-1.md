---
n: 1
title: A map charted and one decision resolved
status: "🔲"
result: false
---
The slice: chart a small real map, resolve its first question with
the human through `/plan-grill`, and watch the answer land as a row of
the map's Phases table. Then flip the map to ✅ with fog still written
down, and watch doctor refuse it. Every later phase copies this
pattern: a skill-text change gated by running the built frit, plus one
doctor finding driven red then green.

Assumes, checked before the first edit:

- The `## ...` slots in [plan/proto.md](../proto.md) admit
  `## Not yet specified` and `## Out of scope`. Check: this plan
  passes `mdsmith check plan/2609280731_decision-maps-from-wayfinder`.
  If it fails, stop: U1 then needs a schema change and the owner's
  consent.
- The Phases catalog renders a result file's `summary` as a `↳` row.
  Check: `mdsmith fix` on a closed folder plan leaves its table
  unchanged. If not, stop and re-plan U3.
- The next free `@C<n>`. Check: read origin's
  [command-scenarios.md](../../docs/research/command-scenarios.md)
  matrix, plus the open plans that reserve rows, at execution time.

RED, each failing on today's code:

1. doctor, given a fixture plan at ✅ whose `## Not yet specified`
   holds a bullet, reports a finding that names the plan and says it
   closed with fog. Given the same plan with the section empty or
   absent, it reports nothing new. An HTML comment alone does not
   count as content.
2. The new `@C<n>` scenario: `frit doctor --json` on that fixture
   carries the finding, and every existing key stays present.

GREEN:

- The finding in doctor, reading the section by its exact heading.
- `plan-grill`, a new skill asset in
  [internal/skills/assets](../../internal/skills/assets). It works in
  rounds. Each round asks every question whose prerequisites are
  settled, numbered, each with a recommended answer. Facts come from
  the tree through a subagent, never from the human. It stops when no
  question is left and the human confirms. It records the human's
  answer in the phase's result file, and its `summary` line states the
  decision. It never answers for the human.
- `plan-new` gains the map path. Name the destination first, with
  `/plan-grill`. Then make one breadth-first pass. If it surfaces no
  open decision, write an ordinary plan. Otherwise write a map: the
  Goal is the destination, each phase file is one sharp question, and
  the dim ones go under `## Not yet specified`.
- `plan-handoff` gains two rules. A closed phase's `summary` states
  its decision, not its activity. Each fog patch the answer cleared
  moves out of `## Not yet specified`, into a new phase file or
  `## Out of scope`, never both.
- The skills narrate plans by title, with the id inside the link
  (U9).
- `plan/proto.md` and the scaffolded proto in
  [internal/scaffold/assets](../../internal/scaffold/assets) name the
  two optional sections and the map shape in their comment blocks.
- Reinstall the dogfood copies with `frit skills . --force --via "go
  run ./cmd/frit"`.

BDD coverage: the doctor finding is command-level, so it adds the
next free `@C<n>` to the matrix and to
[commands.feature](../../features/commands.feature). No `@S<n>`
applies: no claim, takeover or resume changes.

Gate:

- Chart a real map from a foggy question in a scratch repo made by
  `frit init`. Resolve its first question with a human in the loop.
  Then show the rendered Phases table with the answer row.
- Run the built frit's `doctor` against that map flipped to ✅ with
  fog left, and confirm the finding.
- Run each touched skill's commands against the built frit and
  confirm the output matches the claim. Lint and the dogfood-match
  test pass on a false claim.
- Every skill stays within the `skill` kind's token budget.
- `go test ./...`, lint, and `mdsmith check .` are clean.
- The human signs off. The report shows the map, the answer row and
  the doctor finding. It states the candidate paths for U6, U7 and U8,
  what each costs, and which to take next.
