---
n: 2
title: S88 runs for real under godog
status: "✅"
result: false
---
Document the guarantee Phase 1 built as matrix row S88. Write it as a
real godog scenario, so a regression fails the build and the bijection
gate stays satisfied.

**Assumes.** Phase 1 landed the behavior: `pick --go` advances past a
live top lane to the next ready candidate. The bijection gate lives in
[internal/scenario](../../internal/scenario), as
`TestMatrixAndFeaturesAreInBijection`. It requires every `@S` feature
tag to name a documented row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md).
It also requires every documented S row to carry a tagged scenario.
`TestFeatures` in [cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go)
runs each tagged scenario under godog strict mode. It skips a
`@pending` one. Plan
2609021314 (this plan's dependency) has landed the "Cross-layer:
herdr and frit disagree" section as real scenarios and stood up
`cmd/frit/bdd_identity_and_cross_layer_test.go` with its herdr-fake
step vocabulary; Phase 1's handoff names which of that vocabulary S88
reuses and which it must add.

**Value.** The fleet-liveness guarantee stops being a unit test tucked
in `pick_test.go` and becomes a documented promise of the lease
protocol, executable: if a future change makes a live top lane stall
`pick --go` again, S88 fails `go test ./...`, not just a narrowly
named unit test.

**RED.** Add row S88 to the "Cross-layer: herdr and frit disagree"
table in the matrix, in id order after S86 — scenario "live top lane
in pick's walk", outcome and mechanism naming that `pick` advances to
the next ready candidate while an explicit `start` surfaces the
refusal (a herdr/frit disagreement resolved in pick's favor). Add the
`@S88` scenario to
[features/cross-layer.feature](../../features/cross-layer.feature) with
its Given/When/Then, not `@pending`. Run `go test ./cmd/frit -run
'TestFeatures/S88:'`: strict mode reports the new steps undefined and
the subtest fails. `go test ./internal/scenario` is green now that the
matrix row and the tag match. Commit the red.

The scenario, in the matrix's terms. Given a plan whose hold branch
herdr shows a live pane on, and a second plan ready and held by
nobody, when `pick --go` runs, then the second plan is the one
started, and the first is not refused on. The observable is the start
document: which plan's branch stood up. Read it through `--json`, per
the JSON Contract, since the assertion branches on which lane started.

**GREEN.** Bind the scenario's steps in
`cmd/frit/bdd_identity_and_cross_layer_test.go`, reusing the herdr-fake
and ready-plan vocabulary Phase 1's handoff identified and defining
only what S88 adds. A step text the section's file or
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already defines
must not be redefined — strict mode reports it ambiguous. Every new
step function ships with its own unit test (CLAUDE.md).

**Guard the edges.** The `@S88` tag must be the scenario's only S tag,
or the bijection walk reports it. Confirm no other section's step file
already binds a step text this scenario introduces. The scenario must
assert the *started* plan's identity, not merely that a start
happened, so a walk that stalled and refused cannot pass it.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S88:'` passes with
S88 PASS and none SKIP; `go test ./internal/scenario` stays green; `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean; `mdsmith check .` passes.

Write the handoff to `phase-2.result.md`: the S88 row wording as
landed, and any step vocabulary added to the section file.
