---
n: 1
title: plan-phase names the in-lane inference
status: "✅"
result: true
summary: plan-phase's Inputs now names the in-lane inference; dogfood copy regenerated; gate confirmed against the built binary.
---
## Handoff

plan-phase's canonical asset
([internal/skills/assets/plan-phase](../../internal/skills/assets/plan-phase))
gained one clause in Inputs: in the plan's own lane the id is inferred
from the branch, so `{{frit}} phase` needs no selector; pass one only
to act on another plan. The dogfood copy was regenerated with `frit
skills --via "go run ./cmd/frit" --force` — no hand edits.

**Gate confirmed live.** Run from inside this plan's own lane
worktree, `go run ./cmd/frit phase --json` (no selector) resolved
plan 2609061129 — the exact claim the new line makes, checked against
the built binary, not lint.

**Checks green.** `TestDogfoodCopiesMatchCanonical`; `mdsmith check
internal/skills/assets/plan-phase .claude/skills/plan-phase`; `go test
./...`; `go tool -modfile=tools/go.mod golangci-lint run`.

**What the next phase inherits.** The wording, token budget headroom,
and the real-lane gate pattern proved here are ready to copy into
plan-handoff, plan-tidy and plan-drive — each still shows an explicit
`<id>` on every command and never mentions inference. That work is not
phased in this plan; it needs a phase (or a follow-on plan) of its
own.
