---
id: 2609082011
title: A plan-start skill opens the specific plan the caller names
status: "🔲"
summary: >-
  Front start with a thin plan-start skill for an explicit selector.
  Route named starts from plan-pick and plan-drive, install through
  the existing bundle, and prove that the chosen plan starts even
  when another ranks higher. C7 and C8 pin selection and refusal.
model: sonnet
depends-on: []
---
# A plan-start skill opens the specific plan the caller names

## Goal

An agent asked to start a named plan finds a shipped skill that opens
that plan's lane and reports its handoff. Addresses [issue #185][issue].

[issue]: https://github.com/jeduden/frit/issues/185

## Context

No existing plan covers this skill gap. The completed
[skill-front plan][fronts] establishes that agent-facing verbs need a
skill. The completed [selector-inference plan][inference] concerns
in-lane omission of an id, not starting an explicitly selected plan.

[fronts]: ../2608212223_a-skill-fronts-every-verb.md
[inference]: ../2609061129_skills-note-lane-selector-inference/plan.md

Reuse `frit start <selector> --go`: it already resolves the chosen plan,
checks readiness, claims and dispatches. [plan-pick][pick] ranks plans
without a selector; [plan-drive][drive] manages existing lanes and
currently routes fresh claims only to pick. Add a small `plan-start`
skill and correct those routing notes. There is no need to change
pick's selection behavior or add a CLI verb.

[pick]: ../../internal/skills/assets/plan-pick/SKILL.md
[drive]: ../../internal/skills/assets/plan-drive/SKILL.md

Use the existing [skill installer](../../internal/skills/skills.go),
invocation substitution and [bundle tests][tests]. Its canonical asset
lives under [assets](../../internal/skills/assets); regenerate the
repository copy through `frit skills`. Preserve the 650-token budget.

[tests]: ../../internal/skills/skills_test.go

The resume repair in [plan 2609082010][resume] is independent: this plan
proves a fresh unheld start and does not require fixing held-lane resume.
Existing dispatch fencing stays covered by lease scenarios. C7 and C8
in [command scenarios][commands] prove the skill's command contract.

[resume]: ../2609082010_resume-a-started-lane-after-work-commits/plan.md
[commands]: ../../docs/research/command-scenarios.md

## Tasks

1. Add a canonical `plan-start` skill with explicit-selector triggers
   such as "start plan X" and "open a lane for plan X".
2. Teach `{{frit}} start <selector> --go --json`, report the pane when
   `prompt_dispatched` is true, and stop after handoff. A refusal stays
   on the named plan; no fallback to pick or second phase runner.
3. Clarify routing in plan-pick and plan-drive, update the bundle list
   in [development](../../docs/development.md), and regenerate copies.
4. Implement C7 and C8 against the installed command and built binary;
   remove pending labels and verify installation with custom `--via`.

## Execution

| Phase | Title                                | Tier   | Gate                                                                                                                                                         |
| ----- | ------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | Ship and prove the named-start skill | sonnet | Installed command starts the lower-ranked selected plan; blocked selection starts nothing; C7/C8 run; bundle, substitution, token budget and full tests pass |

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

| #   | Status | Phase                                              |
| --- | ------ | -------------------------------------------------- |
| 1   | 🔲     | [Ship and prove the named-start skill](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] Installed plan-start names explicit-plan trigger phrases and
      uses `start <selector> --go --json` with invocation substitution.
- [ ] A named start opens that plan even when another ranks higher;
      the other plan stays unheld, with no agent launched for it.
- [ ] An unmet dependency is reported for the chosen plan without
      starting it or falling back to another ready plan.
- [ ] The skill reads `prompt_dispatched` and the pane from JSON,
      reports the handoff and never launches a second phase runner.
- [ ] plan-pick and plan-drive route named fresh starts to plan-start;
      the documented bundle and regenerated copies include it.
- [ ] C7 and C8 run without `@pending`, using the installed command
      against the built binary in isolated fixtures.
- [ ] Default and custom `--via` installs pass; edited-file refusal
      remains intact and all changed skills meet the token budget.
- [ ] `go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`
      and `mdsmith check .` pass.
