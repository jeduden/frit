---
id: 2609280731
title: Chart a foggy effort as a decision map before building
status: "🔲"
summary: >-
  Borrow Wayfinder's best ideas without its tracker. A plan can be a
  decision map: its Goal is the destination, its phases are questions,
  and it writes down what is still fog and what is out of scope. A new
  grilling skill resolves one question per session with the human. Each
  answer becomes one line in the plan's generated Phases table, and
  frit doctor refuses a plan closed while fog remains. Durable
  decisions graduate to records any later plan can query.
model: sonnet
depends-on: []
---
# Chart a foggy effort as a decision map before building

## Goal

An effort too big and too unclear for one session can be charted in
the plan register as a decision map, and decided one question per
session, before anyone writes a build plan. It keeps frit's claim,
lint and handoff machinery.

## Context

**Why.** Today a plan starts where the route is already clear enough
to write Phase 1. smalt's forward lane answers "how does it play?" by
building a prototype. Nothing answers "what should we decide?" when a
question cannot be played: an architecture choice, a scope line, a
contract. Wayfinder, Matt Pocock's planning skill, is built for exactly
that gap. The comparison, with measured token costs, is in
[wayfinder-comparison.md](wayfinder-comparison.md).

**What is borrowed, and where it lands.** Each idea, its form here,
and the file it changes:

| #   | Wayfinder idea                            | Our form                                                                                         | Lands in      |
| --- | ----------------------------------------- | ------------------------------------------------------------------------------------------------ | ------------- |
| U1  | Fog: a "Not yet specified" section        | An optional `## Not yet specified` in any plan; patches graduate into new phase files            | proto, skills |
| U2  | Out of scope as its own section           | An optional `## Out of scope`, one line and a reason per item                                    | proto         |
| U3  | "Decisions so far": an index, not a store | Each result file's `summary` states the decision; the existing Phases catalog is the index       | plan-handoff  |
| U4  | Grilling as the default ticket            | A `plan-grill` skill: rounds of questions, a recommended answer each, the human decides          | new skill     |
| U5  | Destination named first; no fog, no map   | `plan-new` asks for the destination first and charts a map only when a pass finds open decisions | plan-new      |
| U6  | Typed tickets, HITL or AFK                | `type:` and `mode:` on a phase, so research runs as a subagent and HITL work waits for a person  | later phase   |
| U7  | Plan, don't do                            | doctor flags a map lane whose commits touch code; no switch exists to turn a map into execution  | later phase   |
| U8  | Durable decisions leave the map as ADRs   | Decision records with front matter, found by query, never loaded into every session              | later phase   |
| U9  | Refer by name, never a bare id            | Reports and skills narrate a plan by title, with the id inside the link                          | skills        |

**How decisions stay findable at scale.** This is Wayfinder's weakest
point and U8's reason. A Wayfinder session loads its map's "Decisions
so far" list at low resolution. It opens a ticket only when a line
looks relevant. A decision that must outlive its map moves to an ADR,
one short file in `docs/adr/`, through the `domain-modeling` skill.
Later specs are told to "respect ADRs in the area". Nothing indexes
decisions across maps, and nothing marks a decision superseded except
an optional ADR status. We already have the pieces to do better.
mdsmith queries front matter (`mdsmith list query`). A `<?catalog?>`
turns `summary` lines into an index. frit finds plans by text
(`frit find`). So each decision tier gets its own retrieval:

1. Inside one map: the Phases catalog, one row per answer (U3).
2. Across plans: a decision record kind with `summary`, `areas` and
   `superseded-by` fields, queried by area. A plan's phase bundle
   pulls only the records for its areas (U8).
3. Retired decisions: `superseded-by` set on the old record, and
   doctor flags a live plan that links a superseded one (U8).

**Reuse, searched first.** Each item below was already in the tree:

- The `## ...` slots in [plan/proto.md](../proto.md) already admit
  extra sections. U1 and U2 therefore need no schema change: this plan
  carries both and lints.
- The Phases catalog already renders each `phase-N.result.md`
  `summary` as a `↳` row. U3 is a wording rule on that summary, not
  new machinery. Plan
  [2608310418](../2608310418_phase-front-matter-generated-status/plan.md)
  built the catalog.
- A folder plan already holds companion files. A research answer
  (U6) is a companion file, not a `research/<name>` branch.
- doctor already walks every plan. U1's "closed with fog" check and
  U7's code-touching check are two more findings in that walk. The
  code-touching test already exists as the landed-evidence rule in
  smalt's `docs/planning.md`.
- smalt's `/forward` already resolves prototype questions by playing
  them. A prototype question in a map traces back to its phase, rather
  than gaining a second prototype skill.

**What is not borrowed** is listed under Out of scope below, with the
reasons.

**BDD coverage.** Phase 1 changes one command's output: doctor gains
a finding. That is command-level, so it takes the next free `@C<n>` in
[command-scenarios.md](../../docs/research/command-scenarios.md) at
execution time. No `@S<n>` applies, since no lease behavior changes.

## Tasks

1. Phase 1 is the proving slice: chart a small map, resolve its first
   question with `/plan-grill`, see the answer land as a Phases row,
   and see doctor refuse a map closed with fog left.
2. Later phases are specced from Phase 1's handoff. The candidates
   are under Not yet specified.

## Execution

| Phase | Title                                   | Tier   | Gate                                                                                                                                        |
| ----- | --------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A map charted and one decision resolved | sonnet | the built frit's doctor names a ✅ plan with fog; the new `@C<n>` runs; a fixture map shows its answer row; `go test ./...`; human sign-off |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                 |
| --- | ------ | ----------------------------------------------------- |
| 1   | 🔲     | [A map charted and one decision resolved](phase-1.md) |
<?/catalog?>

## Not yet specified

- **Typed phases (U6).** `type:` and `mode:` on a phase file need the
  `phase-spec` kind's closed front matter widened in `.mdsmith.yml`.
  That needs the owner's consent first. Open: whether `frit phase`
  routes on `type` or only prints it.
- **Plan, don't do (U7).** Which commits count as code in a map lane
  is a repo's choice. The landed-evidence rule's file list is the
  likely default.
- **Decision records (U8).** Where they live: `docs/decisions/` or
  inside plan folders. How `areas` is named without a shared list
  drifting. Whether `frit phase` bundles matching records or only
  names the query.
- **A cleared map becomes a build plan.** The step Wayfinder calls
  `to-spec`: `plan-new` from a map, citing its answers by link and
  listing the map in `depends-on`.
- **smalt adoption.** Tracing a forward finding to a map's phase, the
  orchestrator dispatching only AFK phases unattended, and making the
  human-only skills (forward, trace-forward, orchestrate)
  user-invoked. The last needs a check first that `/loop /orchestrate`
  still fires once the skill's description is removed.

## Out of scope

- The issue tracker as the store. It would lose the atomic claim,
  offline lint and the schema.
- Claiming single questions inside a map for parallel sessions.
  Wayfinder's own guidance is one question at a time. Two parallel
  grills re-ask each other's settled questions.
- Inlining prototype snippets into a plan. smalt's no-paste rule
  stands.
- A notes switch that turns a map into execution. Wayfinder's field
  reports show agents granting it to themselves.

## Acceptance Criteria

- [ ] A foggy effort can be charted as a map plan that passes
      `mdsmith check .` and that frit indexes like any other plan.
- [ ] `/plan-grill` resolves one question with the human per round,
      gives a recommended answer for each question, looks up facts
      itself, and never answers a question on the human's behalf.
- [ ] A resolved question appears as one line in the map's Phases
      table, linking its record, with no index edited by hand.
- [ ] A fog patch the answer cleared is gone from Not yet specified.
      It lives only in its new phase file or under Out of scope.
- [ ] `frit doctor` reports a ✅ plan whose Not yet specified still
      has content.
- [ ] Every shipped skill stays within the `skill` kind's token
      budget, and the bundle gains at most one new skill.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
