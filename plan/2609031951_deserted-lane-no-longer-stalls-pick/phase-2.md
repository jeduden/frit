---
n: 2
title: S90 runs the deserted-lane walk skip under godog
status: "✅"
result: false
---
Document the guarantee Phase 1 built as cross-layer matrix row S90 and
run it for real under godog, so a regression fails the build.

**Assumes.** Phase 1 landed the behavior: `pick --go` advances past a
deserted top lane to the next ready candidate. The bijection gate
`TestMatrixAndFeaturesAreInBijection` lives in
[internal/scenario](../../internal/scenario). It requires every `@S`
tag to name a documented row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md).
It also requires every documented S row to carry a tagged scenario.
`TestFeatures` in [cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go)
runs each tagged scenario under godog strict mode. It skips a
`@pending` one. Plans 2609031211 and 2609031939 (this plan's
dependencies) have landed S88
and S89 in the "Cross-layer: herdr and frit disagree" section and
extended `cmd/frit/bdd_identity_and_cross_layer_test.go`; S90 is the
next free id and lands in that same section, beside S77.

**Value.** The deserted-lane walk skip stops being a unit test in
`pick_test.go` and becomes a documented promise. If a future change
makes a deserted top lane stall `pick --go` again, S90 fails `go test
./...`.

**RED.** Add row S90 to the "Cross-layer: herdr and frit disagree"
table in the matrix, in id order after S89 — scenario "deserted top
lane in pick's walk", outcome and mechanism naming that `pick`
advances to the next ready candidate while an explicit `start` still
refuses with its `frit yield` remedy. Add the `@S90` scenario to
[features/cross-layer.feature](../../features/cross-layer.feature) with
its Given/When/Then, not `@pending`. Run `go test ./cmd/frit -run
'TestFeatures/S90:'`: strict mode reports the new steps undefined and
the subtest fails. `go test ./internal/scenario` is green once the row
and the tag match. Commit the red.

The scenario, in the matrix's terms. Given a plan held as a deserted
hold with an unparked suffix, and a second plan ready and held by
nobody, when `pick --go` runs, then the second plan is the one started
and the first is not refused on. The observable is the start document —
which plan's branch stood up — read through `--json`, per the JSON
Contract, since the assertion branches on which lane started.

**GREEN.** Bind the scenario's steps in
`cmd/frit/bdd_identity_and_cross_layer_test.go`, reusing the deserted-
hold and ready-plan vocabulary Phase 1's handoff identified and
defining only what S90 adds. A step text the section's file or
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already defines
must not be redefined — strict mode reports it ambiguous. Every new
step function ships with its own unit test.

**Guard the edges.** The `@S90` tag must be the scenario's only S tag,
or the bijection walk reports it. The scenario must assert the started
plan's identity, not merely that a start happened, so a walk that
stalled and refused cannot pass it. Confirm no other section's step
file already binds a step text this scenario introduces.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S90:'` passes with S90
PASS and none SKIP; `go test ./internal/scenario` stays green; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are clean;
`mdsmith check .` passes.

Write the handoff to `phase-2.result.md`: the S90 row wording as
landed, and any step vocabulary added to the section file.
