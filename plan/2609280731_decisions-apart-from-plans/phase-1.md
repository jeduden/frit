---
n: 1
title: A decision served where it applies, and only while live
status: "🔲"
result: false
---
The slice exercises the whole lifecycle on one file. Record A governs
it and was made with no plan. Record B supersedes A and names a plan
phase as its source. Record C governs one narrower path and overrides
B there. A session asking about the broad path is served B. A session
asking about the narrow path is served C. Nobody is served A, but A
stays readable as history. A plan that still links A is reported. Every
later phase copies this pattern: a fixture tree of records, a verb run
against the built frit, and a doctor finding driven red then green.

Assumes, checked before the first edit:

- The owner consents to adding a `decision` kind to `.mdsmith.yml`.
  The repo forbids editing that file without consent. If consent is
  withheld, stop: records would lint as untyped Markdown, and the
  schema would have to live in Go alone.
- `frit phase` assembles its bundle in one place that a new section
  can join. Check: read how the handoff section reaches the bundle. If
  the bundle has no such seam, stop and re-plan, rather than adding a
  second assembler.
- The next free `@C<n>` rows. Check origin's
  [command-scenarios.md](../../docs/research/command-scenarios.md)
  matrix, plus open plans that reserve rows, at execution time.

RED, each failing on today's code, against a fixture tree holding A,
B and C:

1. `frit decisions --for <broad path> --json` lists B only. For the
   narrow path it lists C only. An empty result is `[]`, never null,
   and every key is present.
2. `frit decisions show <A>` prints A's body and marks it superseded
   by B. The derived state comes from B's `supersedes`, while A's own
   `status` still reads `accepted`.
3. A proposed record, and a revoked one, are never served as live.
4. `frit phase` on a fixture plan whose phase spec links the broad
   path carries B's summary, and no other record.
5. doctor reports each broken case, and nothing on a clean tree: a
   live plan linking A, a scope matching no file, a `supersedes` id
   that does not exist, a cycle, and an accepted record with no
   `decided-by`.

GREEN:

- A record reader beside the plan reader, keyed by the same id scheme.
- One function computes the live set for a path. Every consumer calls
  it: the verb, the `frit phase` bundle and doctor. No second copy.
- The `decisions` verb, rendered from one model for both the table
  and `--json`, per the JSON contract.
- The doctor findings.
- A `decision-dir` setting, defaulting to `decisions`, written by
  `frit init` with its comment, like `plan-dir`.
- The `decision` kind and a `decisions/proto.md` schema, once the
  owner has consented.
- `/decide`, a new skill asset in
  [internal/skills/assets](../../internal/skills/assets), shipped with
  the verb. Before writing, it runs `frit decisions --for` on the
  scope, so it finds what the new record supersedes or overrides. It
  writes `status: proposed` and asks the human to accept. It never
  fills `decided-by` itself.
- `plan-handoff` gains one line: a phase that settled a decision writes
  a record, rather than burying the decision in its handoff.
- Reinstall the dogfood copies:
  `frit skills . --force --via "go run ./cmd/frit"`.

BDD coverage: the verb and the doctor findings are command-level, so
this phase adds the next free `@C<n>` rows to the matrix and to
[commands.feature](../../features/commands.feature). One row: a path
governed by a superseded and an overriding record is served the live
one only. One row: doctor flags a plan linking a retired record. No
`@S<n>` applies.

Gate:

- Run the built frit against the fixture tree and confirm each RED
  case's output. Lint and the dogfood-match test pass on a false skill
  claim, so the skill's commands are checked this way too.
- Record one real decision in this repo through `/decide`, with the
  human accepting it. Then show `frit decisions --for` serving it.
- `go test ./...`, lint, and `mdsmith check .` are clean. Every skill
  stays within the `skill` kind's token budget.
- The human signs off. The report shows the three records, what each
  path is served, and the doctor findings. It states the candidate
  paths from Not yet specified, what each costs, and which to take
  next.
