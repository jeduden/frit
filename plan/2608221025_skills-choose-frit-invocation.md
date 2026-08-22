---
id: 2608221025
title: The installer chooses how a laid-down skill invokes frit
status: "🔳"
summary: >-
  `frit skills` copies its assets verbatim, so every laid-down skill
  hardcodes a bare `frit` and carries a source-checkout parenthetical
  meant only for the frit repo. A repo that installs frit via mise or a
  local build cannot make the skills call the right binary. Give `frit
  skills` a `--via` flag that substitutes the invocation into a
  templated token, defaulting to `frit`, and drop the frit-repo-only
  hint from the shipped text.
model: sonnet
phases:
  - n: 1
    title: skills substitute a chosen invocation
    status: "✅"
  - n: 2
    title: the shipped guidance names the choice
    status: "🔲"
depends-on: []
---
# The installer chooses how a laid-down skill invokes frit

## Goal

A person laying frit's skills into a repo can say how those skills
should call frit. The choice is one invocation: bare `frit` on `PATH`,
`mise exec -- frit`, a local path, or `go run ./cmd/frit`. So the
installed skill drives the frit that repo actually has.

## Context

The incident behind it: the skills hardcode `frit`. Every laid-down
copy reads `frit pick --go`, and the header carries a parenthetical —
`(source checkout: go run ./cmd/frit)` — that only makes sense inside
the frit source tree, yet it ships into every downstream repo. A repo
that pins frit with mise, or builds it to `./bin/frit`, has no way to
make the skills call that binary. `frit` alone resolves only when a
mise-activated profile has already put the right shim on `PATH`; in a
plain CI shell it does not.

The cause is one copy loop. [Install](../internal/skills/skills.go)
reads each embedded `SKILL.md` and writes its bytes unchanged; the
`frit` in the asset is the `frit` on disk. There is no seam to choose
the invocation.

Reuse, not a new templater: the substitution is a `strings.Replace`
added to the byte between read and write in that same loop, and one
`--via` flag on [skillsCmd](../cmd/frit/main.go) threaded into
`Install`. The assets gain a `{{frit}}` token only where a command is
invoked (`{{frit}} pick`, `{{frit}} board`, `{{frit}} next`); the
prose that names frit the tool — "the claim is a ref frit
force-pushes", "not a frit verb" — stays literal, because it is not a
command. So the substitution never touches a sentence.

The default holds the existing contract.
[TestShippedSkillNamesFritOnPath](../internal/skills/skills_test.go)
pins a default install to read `frit pick`. `--via` defaults to
`frit`, so that test stays green. The dogfooded `.claude/skills` in
this repo are the source checkout. They regenerate with `--via "go run
./cmd/frit"`, and the awkward dual-form header collapses to one clean
line. That is also how the frit-repo-only parenthetical leaves the
shipped text.

Persisting the choice in `.frit.yml` so a re-run need not repeat it is
a natural follow-up, but out of scope here; the flag is the seam.

## Tasks

1. `Install` substitutes a chosen invocation for a `{{frit}}` token;
   `frit skills --via` supplies it, defaulting to `frit`.
2. The shipped guidance and the dogfooded copies name the choice.

## Phase 1: skills substitute a chosen invocation

`Install` gains an `invoke` parameter. It replaces every `{{frit}}`
token in each asset's bytes with `invoke` before writing, so the
laid-down skill invokes whatever the caller named. An empty `invoke`
means bare `frit`. The four assets are edited so each command span
reads `{{frit}} <verb>` while every prose mention of frit stays a bare
word. `frit skills` grows a `--via` flag, default `frit`, threaded
into `Install`. The dogfooded `.claude/skills` are regenerated with
`--via "go run ./cmd/frit"`, which drops the source-checkout
parenthetical from those copies.

RED, extending [skills_test.go](../internal/skills/skills_test.go)
against the `Install(...)` / `os.ReadFile` idiom already there:

- `Install(dir, false, "mise exec -- frit")`: the plan-pick skill's
  command reads `mise exec -- frit pick`, not `frit pick`.
- The same install leaves a prose mention of frit — the word in "frit
  force-pushes" — as a bare `frit`, not `mise exec -- frit
  force-pushes`.
- `Install(dir, false, "")` and `Install(dir, false, "frit")`: the
  command reads `frit pick`, keeping `TestShippedSkillNamesFritOnPath`
  green.

GREEN: add the `invoke` parameter and the `strings.Replace` in
[Install](../internal/skills/skills.go). Default and thread the `--via`
flag in [skillsCmd](../cmd/frit/main.go). Put `{{frit}}` in the command
spans of the four assets. Rewrite the plan-pick header to a single
templated line. Regenerate the dogfooded copies with `go run ./cmd/frit
skills --force --via "go run ./cmd/frit"`.

Gate: the RED cases pass; `TestShippedSkillNamesFritOnPath` still
passes; `go test ./...` and `go vet ./...` are clean; `mdsmith check .`
passes on both the tokened assets and the regenerated copies.

## Phase 2: the shipped guidance names the choice

The "Shipping Skills" section of [CLAUDE.md](../CLAUDE.md) still says a
shipped skill names `frit` because it lands on `PATH`. Extend it to
record the `--via` seam: the default is `frit`, a mise-pinned or
locally-built repo passes its own invocation, and the dogfooded copies
regenerate with `--via "go run ./cmd/frit"`. The `frit skills --help`
text names what `--via` is for. It also shows the invocations a reader
would actually pass: `frit`, `mise exec -- frit`, `go run ./cmd/frit`.
So the flag teaches its own use rather than describing it abstractly.

RED: this phase is documentation and flag help, so the gate is the
linter and a help-text assertion. Add a test that `frit skills --help`
output mentions `--via` and carries the `mise exec -- frit` example.

GREEN: edit the CLAUDE.md section and the flag's `help:` tag, putting
the example invocations in the help.

Gate: the help test passes; `go test ./...` is clean; `mdsmith check .`
passes.

## Execution

Tier is per phase. The design is settled here; Phase 1 implements from
written assertions guarded by unit tests over `Install`, and Phase 2
is prose plus a help string.

| Phase          | Design | Implement | Gate that catches a wrong answer                                           |
| -------------- | ------ | --------- | -------------------------------------------------------------------------- |
| 1 substitution | opus   | sonnet    | `--via` value appears in a command span, never in prose; default is `frit` |
| 2 guidance     | opus   | sonnet    | `--help` names `--via`; `mdsmith check .` is clean                         |

## Acceptance Criteria

- [ ] `frit skills --via "mise exec -- frit"` writes skills whose
      commands read `mise exec -- frit <verb>`
- [ ] A prose mention of frit in a skill stays a bare word after any
      `--via` value
- [ ] `frit skills` with no `--via` writes `frit <verb>`, and
      `TestShippedSkillNamesFritOnPath` stays green
- [ ] The shipped assets no longer carry the source-checkout
      parenthetical; the dogfooded copies regenerate cleanly
- [ ] `CLAUDE.md` and `frit skills --help` name the `--via` seam, and
      the help shows example invocations (`frit`, `mise exec -- frit`,
      `go run ./cmd/frit`)
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
