---
id: 2608192121
title: init scaffolds the plan machinery, not just the config
status: "🔲"
summary: >-
  frit init writes .frit.yml, but the plan workflow it enables assumes
  a plan/proto.md schema and a PLAN.md index that a fresh repo has not
  got. The plan-new skill and frit doctor both validate against
  proto.md. Ship that machinery from init so a repo adopting frit has
  the conventions its skills and checks depend on.
model: sonnet
depends-on: []
---
# init scaffolds the plan machinery, not just the config

## Goal

Make `frit init` lay down the plan machinery a repo needs to use
frit's workflow — the `plan/proto.md` schema and a `PLAN.md` index —
not only the `.frit.yml` config. A repo that ran `init` then has the
conventions the shipped skills and `frit doctor` assume, rather than a
config pointing at a plan directory that does not exist.

## Context

[repocfg.Init](../internal/repocfg/template.go) writes one file,
`.frit.yml`, and refuses to clobber it without force. That is the only
thing `init` scaffolds. But the plan workflow needs more to exist: the
`plan-new` skill authors files "conforming to `plan/proto.md`", and
`frit doctor` (plan 2608192045) validates plans against that same
schema through mdsmith. A repo with a `.frit.yml` but no `plan/` gets a
config that promises a convention nothing on disk carries.

frit already ships files it owns into a repo. `init` writes the config
template, and `frit skills` embeds and installs skills, refusing to
clobber an edit. `plan/proto.md` and a `PLAN.md` skeleton are the same
class of shipped default: embedded in the binary, written once,
editable after.

### Reuse

The embed-and-install pattern is settled in `internal/skills`: a
`go:embed` asset, an `Install`
that writes it and refuses to clobber without force, and a dogfood
check that frit's own copy matches what ships. This plan reuses that
shape rather than inventing a second one. The canonical `plan/proto.md`
already lives in this repo; the shipped copy is embedded from an asset
and pinned equal to it, exactly as the skills are.

## Non-goals

- frit does not own a repo's plans, only the defaults. The scaffolded
  files are editable; `init` writes them once and never rewrites an
  edited one without force.
- Not a plan format change. proto.md is shipped as it already is here;
  this plan moves it, it does not redesign it.

## Phase 1: init writes proto.md

The proving slice: `frit init` also writes `plan/proto.md` from an
embedded copy of frit's canonical schema, into the repo's configured
plan directory, refusing to clobber an existing one without force. The
written paths are reported, and the `--json` document carries them.
This establishes the seam — embed the schema, write it beside the
config, report it — that the PLAN.md index and any later default copy.
It ends in sign-off on shipping proto.md from init.

## Tasks

1. Phase 1 — proving slice: `frit init` writes an embedded
   `plan/proto.md` into the plan directory, refusing to clobber without
   force, then sign-off.
2. (determined after Phase 1 sign-off)

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase        | Design | Implement | Gate that catches a wrong answer                         |
| ------------ | ------ | --------- | -------------------------------------------------------- |
| 1 ship proto | opus   | sonnet    | test that init writes proto.md and refuses to clobber it |

## Acceptance Criteria

- [ ] `frit init` writes `plan/proto.md` into the configured plan
      directory, and reports the path
- [ ] A second run without `--force` refuses to clobber an edited
      proto.md; `--force` overwrites
- [ ] The shipped proto.md is pinned equal to this repo's
      `plan/proto.md`, so it cannot drift from what frit lints against
- [ ] `frit init --json` carries every written path, `[]` never null
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
