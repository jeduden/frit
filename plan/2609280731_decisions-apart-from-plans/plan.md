---
id: 2609280731
title: Decisions live apart from plans and reach the session that needs them
status: "🔲"
summary: >-
  A decision gets its own record and its own lifecycle, apart from any
  plan. A plan may produce none or many, and a decision may be made
  with no plan at all. A record is proposed, then accepted by a human,
  and later superseded or overridden by another record rather than
  edited. Each record names the paths it governs, so frit serves the
  live decisions for exactly the files a session touches. Ideas taken
  from Wayfinder, kept on frit's register.
model: opus
depends-on: []
---
# Decisions live apart from plans and reach the session that needs them

## Goal

Every accepted decision is one record with its own lifecycle. A
session that is about to touch a file is served the decisions still
live for that file, and none of the retired ones. It gets them without
loading every decision, and without knowing which plan made them.

## Context

**Why a plan is the wrong home.** A plan and a decision have different
lives:

- A plan produces none, one or many decisions.
- A decision can be made with no plan: in a review, a conversation, or
  a forward session.
- A decision must be read where it applies, which is a place in the
  tree. It does not live in the history of the plan that made it.
- A decision can be overridden by a later one. A plan is closed and
  stays closed.

A plan ends at ✅. A decision stays in force until another decision
retires it. Plans are indexed by their state of work; decisions must
be indexed by where they apply. So this plan gives decisions their own
record, their own lifecycle and their own retrieval. Plans only link
to them.

**Where Wayfinder stops.** Wayfinder, Matt Pocock's planning skill,
keeps a map's "Decisions so far" list inside the map. It moves lasting
decisions to ADRs, one short file each in `docs/adr/`. Later work is
told to "respect ADRs in the area", but nothing finds them for you.
Nothing searches across maps, and an ADR is marked superseded only by
an optional status field. The full comparison is in
[wayfinder-comparison.md](wayfinder-comparison.md).

**The record.** One file per decision, under a `decisions/` directory
beside `plan/`. Its id uses the plan id scheme
(`date -u +%y%m%d%H%M`). Front matter:

| Field        | Holds                                                                           |
| ------------ | ------------------------------------------------------------------------------- |
| `summary`    | the decision in one line: what a session is served                              |
| `status`     | `proposed`, `accepted` or `revoked`; superseded is never typed, it is derived   |
| `decided-by` | the person who accepted it; empty while proposed                                |
| `scope`      | path globs the decision governs; each must match a file in the tree             |
| `supersedes` | ids this record retires entirely                                                |
| `overrides`  | ids this record beats inside its own, narrower scope; outside it they stay live |
| `source`     | where it came from: a plan and phase, a forward finding, a PR, or `direct`      |

The body holds the decision, why it was taken, and the alternatives
that lost, in a few sentences like an ADR.

**The lifecycle.** A record moves through these states:

| State      | Entered by                                       | Who                  | Served to a session              |
| ---------- | ------------------------------------------------ | -------------------- | -------------------------------- |
| proposed   | a record is written with `status: proposed`      | a person or an agent | no; listed as open               |
| accepted   | `status: accepted` and `decided-by` set          | a person only        | yes, where its scope matches     |
| superseded | a later accepted record lists it in `supersedes` | derived, never typed | only as history, on request      |
| overridden | a later accepted record lists it in `overrides`  | derived, per path    | not inside the overrider's scope |
| revoked    | `status: revoked`, with the reason in the body   | a person only        | no                               |

An accepted record is never rewritten in substance. A change is a new
record that supersedes or overrides it. This is the same append-only
rule plan phases follow.

**Serving the right context.** A new verb, `frit decisions`, answers
one question: which decisions are live for these paths?

- `frit decisions --for <path>...` lists the accepted, unsuperseded
  records whose scope matches, minus any a matching record overrides.
  The table shows the summaries; `--json` is for an agent to branch on.
- `frit phase` adds the same list for the paths its phase spec links.
  An executor gets the decisions for the files it will touch, and
  nothing else.
- `frit decisions show <id>` gives one record's body, and its chain of
  predecessors on request.

This is Wayfinder's "index at low resolution, open on demand". The
index is scoped by path rather than by map, and it is never loaded
into every session.

**How plans relate.** A decision's `source` names the plan and phase
that produced it. Backlinks from the plan are derived by query, not
listed twice. A plan relies on a decision by linking its record. doctor
reports a live plan that links a superseded or revoked record, so a
plan cannot quietly build on a retired decision.

**What the tree can check.** A scope glob that matches no file means
the decision governs code that is gone. doctor reports it for review.
A `supersedes` or `overrides` id must exist and must not form a cycle.
An accepted record must name who accepted it.

**Reuse, searched first.** Each of these is already in the tree:

- The plan id scheme and doctor's id-sync, for decision ids.
- The `plan-dir` setting in `.frit.yml`, as the pattern for a
  `decision-dir` setting.
- The `phase-spec` and `phase-record` kinds in `.mdsmith.yml`, as the
  pattern for a `decision` kind. Adding a kind changes the linter
  configuration, so it needs the owner's consent first.
- The `frit phase` bundle, which already assembles one session's
  context. Decisions become one more section of it.
- doctor's walk over every plan, which gains the decision findings.

**What else is taken from Wayfinder.** These ideas still help. They are
candidates for later phases, listed under Not yet specified:

- A plan can carry `## Not yet specified` (fog) and `## Out of scope`
  sections. The `## ...` slots in [plan/proto.md](../proto.md) already
  admit them, and this plan carries both and lints.
- A grilling skill: rounds of questions, a recommended answer for
  each, and the human decides. Its output is decision records.
- A decision map: a plan whose phases are questions. It produces
  records and no code. Its "Decisions so far" is a query on `source`.
- Typing phases as human-led or agent-led, and research as a subagent.

**BDD coverage.** Phase 1 adds a verb and doctor findings. Both are
command-level, so it takes the next free `@C<n>` rows in
[command-scenarios.md](../../docs/research/command-scenarios.md) at
execution time. No `@S<n>` applies: nothing here claims or leases.

## Tasks

1. Phase 1 is the proving slice. Two records govern the same file:
   the second supersedes the first, and a third, narrower one
   overrides part of it. `frit decisions --for` serves exactly the
   live one, `frit phase` carries it, and doctor flags a plan still
   linking the retired one. `/decide` ships with the verb.
2. Later phases are specced from Phase 1's handoff. The candidates are
   under Not yet specified.

## Execution

| Phase | Title                                                   | Tier | Gate                                                                                                                                 |
| ----- | ------------------------------------------------------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | A decision served where it applies, and only while live | opus | the built frit serves the live record for a path and not the retired ones; the new `@C<n>` rows run; `go test ./...`; human sign-off |

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

| #   | Status | Phase                                                                 |
| --- | ------ | --------------------------------------------------------------------- |
| 1   | 🔲     | [A decision served where it applies, and only while live](phase-1.md) |
<?/catalog?>

## Not yet specified

- **Topics as well as paths.** Some decisions govern a process, not a
  file: how releases are cut, how plans are written. Open: a `topics`
  field queried by name, or a scope on the doc that describes the
  process.
- **Serving at edit time.** Beyond `frit phase`, a harness hook could
  serve the decisions for a file just before an agent edits it. Open:
  whether that is worth the extra tokens on every edit.
- **Grilling and decision maps.** A `plan-grill` skill and a map plan
  whose phases are questions, writing records as they are answered.
  Fog and out-of-scope sections in plans come with them.
- **Carrying existing decisions over.** smalt's docs record decisions
  inline, such as the dated decisions in its forward-lane doc. Open:
  whether to migrate them, or record only new ones.
- **Decisions across repositories.** frit indexes many repos. A
  decision in one repo can bind another, for example a JSON contract
  that smalt's skills rely on.
- **smalt adoption.** A forward finding with a `keep` verdict may carry
  a decision as well as a need. The orchestrator could dispatch only
  agent-led phases unattended.

## Out of scope

- Storing decisions in the issue tracker. It would lose offline lint,
  the schema and review through pull requests.
- Detecting that two live decisions contradict each other in meaning.
  The tree can check scopes and links, not intent. A person resolves
  a clash by writing a record that overrides one.
- Editing an accepted record in place. A change is always a new record.
- Letting an agent accept a decision. It may propose one; a person
  accepts it.

## Acceptance Criteria

- [ ] A decision can be recorded with no plan, and a plan can produce
      several, each naming its source.
- [ ] `frit decisions --for <path>` lists only accepted records whose
      scope matches the path, excluding superseded ones and any that a
      matching record overrides.
- [ ] `frit phase` carries the live decisions for the paths its phase
      spec links, and nothing more.
- [ ] A superseded record is still reachable as history through
      `frit decisions show`, and is never served as live.
- [ ] doctor reports a live plan that links a retired record, a scope
      that matches no file, a missing or cyclic `supersedes` or
      `overrides` id, and an accepted record with no `decided-by`.
- [ ] `/decide` ships in the same change as the verb, within the
      `skill` kind's token budget.
- [ ] `--json` keeps the contract: every key present, lists never null.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
