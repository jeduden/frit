---
id: 2608221227
title: 'Guard the skill bundle: no drift, tiny, bounded in the binary'
status: '🔲'
summary: >-
  Pin the dogfooded .claude/skills copies byte-equal to the canonical
  assets, cap each skill's token budget, and tighten every suitable
  mdsmith rule so an embedded skill stays lean.
model: 'sonnet'
depends-on: []
phases:
  - n: 1
    title: 'Pin the dogfood copies byte-equal to canonical'
    status: '🔲'
  - n: 2
    title: "Cap each skill's token budget"
    status: '🔲'
  - n: 3
    title: 'Tighten every suitable lean rule'
    status: '🔲'
---
# Guard the skill bundle: no drift, tiny, bounded in the binary

## Goal

A shipped skill must not drift from its dogfooded copy. It must not
grow token-fat. And the suite must not bloat the binary unnoticed.
Every skill is embedded in the binary and loaded into an agent's
context, so each one is paid for twice.

## Context

frit ships four Claude Code skills. The canonical text lives in
[internal/skills/assets](../internal/skills/assets). `frit skills`
in [internal/skills/skills.go](../internal/skills/skills.go) copies
the embedded bytes verbatim into a repo's `.claude/skills/`. frit's
own [.claude/skills](../.claude/skills) is that same output, dogfooded
and checked in.

Three gaps motivate this plan:

- **Drift.** [CLAUDE.md](../CLAUDE.md) claims the dogfood copy "never
  drifts", but nothing enforces it. The two trees match today only by
  hand. An edit to an asset that skips `frit skills` goes unnoticed.
- **Token weight.** The `skill` kind in [.mdsmith.yml](../.mdsmith.yml)
  caps lines but not tokens. A skill loads into a working session, so
  tokens are the real cost, not lines.
- **Binary footprint.** `//go:embed all:assets` bakes every skill into
  the binary, and each one also loads into an agent's context. mdsmith
  bounds per file, not per total, so the mitigation is to make every
  skill as lean as the linter can force — so the suite growing stays
  cheap.

Machinery searched for reuse:

- **Anti-drift precedent.** `TestShippedProtoMatchesRepo` in
  [scaffold_test.go](../internal/scaffold/scaffold_test.go) pins the
  embedded proto byte-equal to `plan/proto.md`. Phase 1 mirrors it.
- **mdsmith rules.** MDS028 `token-budget` is the exact token guard,
  adopted in Phase 2. MDS021 `include` was rejected: `frit skills`
  copies raw bytes and an agent reads the file literally, so directive
  markers would be context noise. MDS020 `required-structure` checks
  shape, not text, so it cannot catch prose drift. No mdsmith rule
  enforces whole-file identity between two plain files, which is why
  Phase 1 is a Go pin, not a rule.
- **The bundle walker.** `bundledFiles()` already lists every asset as
  a slash path. Phase 1 iterates it rather than re-walking.

## Tasks

1. Pin the dogfood copies byte-equal to canonical.
2. Cap each skill's token budget with mdsmith.
3. Tighten every other suitable mdsmith rule on the skill kind.

## Phase 1: Pin the dogfood copies byte-equal to canonical

**RED.** Add `TestDogfoodCopiesMatchCanonical` to
[skills_test.go](../internal/skills/skills_test.go). For each path
from `bundledFiles()`, read the canonical asset and the checked-in
copy at `filepath.Join("..", "..", ".claude", "skills", rel)`, then
assert the two byte slices are equal. Prove the guard bites: append
one byte to a `.claude/skills/*/SKILL.md`, run the test, watch it
fail.

**GREEN.** Revert the drift so every copy matches. The test passes.
This enforces the anti-drift the prose already promises, the same way
the embedded proto is pinned.

**Sites.** [skills_test.go](../internal/skills/skills_test.go) for the
test. The "Shipping Skills" section of [CLAUDE.md](../CLAUDE.md) to
name the pin.

**Gate.** `go test ./internal/skills` fails on any drift between the
two trees and passes when they match.

## Phase 2: Cap each skill's token budget

**RED.** Add MDS028 `token-budget` to the `skill` kind in
[.mdsmith.yml](../.mdsmith.yml), with `mode: heuristic` and
`max: 650`. Prove it bites: append filler prose to
[plan-phase/SKILL.md](../internal/skills/assets/plan-phase/SKILL.md)
until it crosses 650 tokens, run `mdsmith check .`, confirm MDS028
fires on it.

**GREEN.** Remove the filler. Every real skill is under 650 heuristic
tokens today. The largest, plan-phase, is about 552, so
`mdsmith check .` is clean. Keep the line cap as the coarse guard;
token-budget is the token-efficiency one. Record the budget in the
`skill` kind comment and in [CLAUDE.md](../CLAUDE.md).

**Sites.** [.mdsmith.yml](../.mdsmith.yml) `skill` kind.
[CLAUDE.md](../CLAUDE.md) Shipping Skills. The dogfood copies are
linted by the same kind, so Phase 1's pin keeps them identical.

**Gate.** `mdsmith check .` fails when a skill exceeds the budget and
is clean otherwise.

## Phase 3: Tighten every suitable lean rule

The token budget bounds each skill by count. This phase adds every
other mdsmith rule that forces a skill to earn its bytes, so the
footprint the binary carries stays minimal as the suite grows. Most
land on the `skill` kind in [.mdsmith.yml](../.mdsmith.yml) at the
strictest threshold the four current skills still pass with modest
headroom; `directory-structure` is global and goes in top-level
`rules`.

Suitable rules, each driven red by a deliberate violation, then green:

- **MDS036 max-section-length** — cap section lines with `per-level`,
  so no `## Method` sprawls. The longest today is about 30 lines.
- **MDS024 paragraph-structure** — pin sentence and word caps per
  paragraph on the skill kind rather than leaning on the default.
- **MDS056 forbidden-text** and **MDS055 forbidden-paragraph-starts**
  — ban token-wasteful filler ("in order to", "please note", "it is
  important to", "as mentioned"), so prose stays dense.
- **MDS022 max-file-length** — tighten the cap from 80 toward the
  real ceiling; the largest skill is 56 lines.
- **MDS033 directory-structure** — declare the allowed markdown homes
  so a stray `.md` dropped under `assets/` (which `all:embed` would
  bake into the binary) fails the lint. This is a global rule, not
  kind-scoped, so the allowed list must name every markdown home in
  the repo, not just the skills tree. It is directory-granular: it
  stops a markdown file in an undeclared place, but a non-`SKILL.md`
  markdown file inside an already-allowed skill dir still passes. A
  non-markdown file embedded by `all:assets` stays outside mdsmith's
  reach entirely; naming that boundary is the honest record.

Explicitly rejected, recorded in the `skill` kind comment:

- **MDS037 duplicated-content** — the dogfood copies are byte-identical
  to the assets by design, so it would false-fire across the suite.

**RED.** For each kind-scoped rule, introduce a violation in a skill
asset (an over-long section, a filler phrase) and confirm
`mdsmith check .` fires that rule's id. For `directory-structure`,
drop a stray `notes.md` under `internal/skills/assets` and confirm it
fires, having first declared every existing markdown home so nothing
already in the repo trips it.

**GREEN.** Remove the violations; the four real skills and the rest of
the repo pass. Record the adopted rules and the one reject, with
reasons, in the `skill` kind comment and in [CLAUDE.md](../CLAUDE.md).

**Sites.** [.mdsmith.yml](../.mdsmith.yml) `skill` kind.
[CLAUDE.md](../CLAUDE.md) Shipping Skills. Phase 1's pin keeps the
dogfood copies, linted by the same kind, identical.

**Gate.** `mdsmith check .` fires the matching rule id on any skill
that breaks a lean rule, and is clean otherwise.

## Execution

Tier is per phase. This plan fixes the design, so each phase
implements from a written assertion. A loud gate — a Go test or
`mdsmith check` — catches a wrong answer cheaply.

| Phase               | Design | Implement | Gate that catches a wrong answer                           |
| ------------------- | ------ | --------- | ---------------------------------------------------------- |
| 1 byte-equal pin    | opus   | sonnet    | `go test ./internal/skills` fails on any two-tree drift    |
| 2 token budget      | sonnet | sonnet    | `mdsmith check .` fires MDS028 over budget, clean under it |
| 3 lean-rule battery | sonnet | sonnet    | `mdsmith check .` fires the rule id on any un-lean skill   |

## Acceptance Criteria

- [ ] Drift between an asset and its `.claude/skills` copy fails
      `go test ./internal/skills`
- [ ] A skill over 650 heuristic tokens fails `mdsmith check .`
- [ ] Every suitable lean rule is on the skill kind; a violating skill
      fails `mdsmith check .`
- [ ] A stray `.md` under `internal/skills/assets` fails
      `mdsmith check .` via directory-structure
- [ ] The `skill` kind comment records the adopted rules and the one
      rejected one with reasons
- [ ] CLAUDE.md's Shipping Skills section names all three guards
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
