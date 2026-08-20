---
id: 2608192121
title: init scaffolds the plan machinery, not just the config
status: "✅"
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

Make `frit init --mdsmith` lay down the mdsmith machinery a repo needs
to use frit's workflow — a default `.mdsmith.yml`, the `plan/proto.md`
schema, and a `PLAN.md` index. It all depends on mdsmith to be of
value: without the config proto.md does not even lint, and PLAN.md is a
catalog mdsmith regenerates. So a plain `init` writes only `.frit.yml`,
frit's own config, and never seeds a file the repo cannot keep correct
without mdsmith. A repo that ran `init --mdsmith` then has the
conventions the shipped skills and `frit doctor` assume, and lints
clean out of the box.

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
- frit does not generate the PLAN.md catalog. It imports mdsmith's
  parser, not its catalog builder, so `init` ships a static empty seed;
  mdsmith fills and maintains it from then on.

## Phase 1: the proto write seam

The proving slice: a `scaffold` package writes `plan/proto.md` from an
embedded copy of frit's canonical schema into a plan directory,
creating it if absent, refusing to clobber an existing one without
force. The shipped copy is pinned equal to this repo's `plan/proto.md`.
This establishes the seam — embed a default, write it, report it,
refuse to clobber — that the flag and the PLAN.md seed reuse.

## Phase 2: the --mdsmith flag gates the machinery

A plain `frit init` writes only `.frit.yml`. `frit init --mdsmith`
additionally writes `plan/proto.md`, through the Phase 1 seam. The flag
defaults off, so `init` never seeds a file that needs mdsmith to be of
value. Every written path is reported, and the `--json` document
carries them all, `[]` never null.

## Phase 3: --mdsmith completes the machinery

`frit init --mdsmith` also writes a default `.mdsmith.yml` and a
`PLAN.md` catalog seed. The config is what makes the machinery lint at
all — without it proto.md trips MDS020, since nothing marks it as a
schema. The seed is the empty skeleton mdsmith renders for a repo with
no plans yet, carrying the `<?catalog?>` directives it fills as plans
accrue. Each refuses to clobber an edit without force, and `mdsmith
check` passes on a freshly `--mdsmith`-inited repo.

## Tasks

1. Phase 1 — the write seam: a `scaffold` package writes an embedded
   `plan/proto.md`, refusing to clobber without force, pinned equal to
   this repo's copy.
2. Phase 2 — `--mdsmith` gates the machinery: plain `init` writes only
   `.frit.yml`; `--mdsmith` adds proto.md.
3. Phase 3 — `--mdsmith` writes a default `.mdsmith.yml` and an empty
   `PLAN.md` catalog that mdsmith check accepts and later regenerates.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase          | Design | Implement | Gate that catches a wrong answer                           |
| -------------- | ------ | --------- | ---------------------------------------------------------- |
| 1 proto seam   | opus   | sonnet    | test that WriteProto writes proto.md and refuses clobber   |
| 2 mdsmith flag | opus   | sonnet    | test that plain init writes only .frit.yml, flag adds it   |
| 3 PLAN.md seed | opus   | sonnet    | test that mdsmith check passes on an --mdsmith-inited repo |

## Acceptance Criteria

- [x] A plain `frit init` writes only `.frit.yml`
- [x] `frit init --mdsmith` writes `.mdsmith.yml`, `plan/proto.md` and
      `PLAN.md` into the repo, and reports each path
- [x] A second run without `--force` refuses to clobber an edited
      scaffolded file; `--force` overwrites
- [x] The shipped proto.md is pinned equal to this repo's
      `plan/proto.md`, so it cannot drift from what frit lints against
- [x] The PLAN.md seed is the empty catalog mdsmith renders for a repo
      with no plans, so `mdsmith check` passes on a freshly-inited repo
- [x] `frit init --json` carries every written path, `[]` never null
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
