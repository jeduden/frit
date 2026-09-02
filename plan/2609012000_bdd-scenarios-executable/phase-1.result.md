---
n: 1
title: The godog harness runs one real scenario, and the coverage gate binds the matrix
status: "✅"
result: true
summary: >-
  godog runs from `go test ./...` in cmd/frit — the one package where
  the lease API, the repo fixtures, the herdr fake and the verbs all
  meet — over nine feature files, one per matrix section, all 87 S-ids
  tagged: S16 (host resurrected days later -> FENCE, sibling history,
  every push rejected, YIELD) is fully written over the claim API and
  the cmd/frit fixtures, and every other id is declared `@pending` and
  skipped, never run. Each scenario is its own subtest named by id,
  strict mode fails an undefined step, and internal/scenario's
  bijection gate reads the matrix through the markdown parser and the
  features through godog's own, failing loud on a malformed, duplicate
  or missing id on either side.
---
## Handoff

Building the gate surfaced a real bug before any harness code ran:
the matrix had two unrelated rows both claiming S86 — the own-token
re-anchor scenario (plan 2608231006, cited by four Go comments) and a
later Gather-fetch scenario added independently, unaware of the
collision. A duplicate is loud, not silently collapsed, so this would
have blocked the gate forever; the second row is renumbered S87, its
one cross-reference (S80's note) updated, and the row itself says it
was S86, since commit 85cee2e and PR #79 still cite it by the old id.

The gate reads both sides through real parsers, never by splitting
lines. The matrix side takes every table whose header leads with `#`
— the shape the S, F and A tables share — through the same markdown
seam mdsmith uses, so a glossary whose first column starts with "S", a
pipe row quoted in a fenced block, or a bare id in prose never counts,
while every row inside an id table must lead with a clean S/F/A id: a
lowercase, suffixed, zero-padded or shifted one is reported with its
line. The feature side asks godog for the scenarios it would run —
the same recursive walk, the same Gherkin — and requires exactly one
`@S` tag per scenario, so a tag inherited from the Feature line, a
docstring that looks like a tag, or a nested directory reads the same
to the gate and the runner. Verified live: a scratch row with no
scenario turns the gate red, an undefined step fails the suite, and a
scenario that loses its `@pending` tag without gaining steps fails.

Feature files are split per matrix section (`features/process-death
.feature`, `host-death.feature`, `partitions.feature`, `races.feature`,
`clocks.feature`, `storage.feature`, `identity.feature`,
`lifecycle.feature`, `cross-layer.feature`), so each later batch can
work one file. A declared-but-unwritten scenario carries `@pending`
beside its id and no steps; `TestFeatures` skips it, so `go test -v`
shows it as SKIP rather than a pass, and runs every other scenario as
its own subtest — `go test ./cmd/frit -run TestFeatures/S16` picks one
out. Converting a scenario means dropping `@pending`, writing its
Given/When/Then, and binding the step functions in the section's own
`cmd/frit/bdd_<section>_test.go`, appended to the step registry from
`init` as `bdd_lease_test.go` is — a section adds a file, never a line
to `bdd_test.go`, so sections land in any order. Every registrar
binds on the one world a scenario threads, so a section's step reads
what a reused lease step set up, and keeps its own state in a struct
reached through `section[T]`; a step whose text
matches nothing fails under godog's strict mode instead of passing as
undefined, and one two sections both define fails as ambiguous.

The harness lives in cmd/frit, not internal/claim: claim does not
import herdr by design (the veto lives above the lease atom), and
cmd/frit is package main, so its verbs, its herdr fakes and its repo
fixtures are unimportable from anywhere else — most matrix rows name
one of them. Each scenario runs godog on its own subtest's goroutine
with godog's per-scenario subtests off, so the world holds the real
`*testing.T` the fixtures need and a failing fixture fails exactly
that scenario. S16 drives the exported lease API — acquire, takeover,
renew, yield, the remote tip read — over `claimableRepo` and a
`cloneAgain` added beside it; every quoted machine name in a step is
checked against its role, so a scenario cannot pass by naming the
wrong box. The herdr fake is still unexercised: a session-liveness
scenario (S6, S61, S65, S73) is the first to wire `withHerdr` and
`startHerdr` into steps.

godog is a direct require of the main module and, with the Gherkin
message types, is imported by internal/scenario's non-test code. That
makes it a build dependency of an internal package only; a consumer's
`go list -m all` does list it — module pruning skips compiling a
test-only require, it does not drop it from minimal version selection
— which is acceptable because nothing frit exports reaches it.
`tools/go.mod` isolates tools with their own version floors, which
this is not.

Open after this phase: the plan's second task, converting the 86
pending scenarios in themed batches, has no phase spec of its own, so
the plan reads ✅ while its goal is one scenario deep — a successor
plan, or further phases here, must carry it. The branch's red commit
(06830b4) does not lint on its own: it flipped this plan's status
without regenerating PLAN.md, so a bisect of `mdsmith check .` lands
on it.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
