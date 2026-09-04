---
n: 3
title: S89 runs the attended-lane state end-to-end under godog
status: "✅"
result: false
---
Document the attended-lane state as cross-layer matrix row S89. Run it
for real under godog, so the contradiction Phases 1 and 2 removed
cannot return unnoticed.

**Assumes.** Phases 1 and 2 landed the behavior: board does not call an
attended lane dead, and the deserted refusals name the pane and lead
with resume. The bijection gate `TestMatrixAndFeaturesAreInBijection`
lives in [internal/scenario](../../internal/scenario). It requires
every `@S` tag to name a documented row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md).
It also requires every documented S row to carry a tagged scenario.
Plan 2609031211 (this plan's dependency) has landed S88 in the
"Cross-layer: herdr and frit disagree" section and extended
`cmd/frit/bdd_identity_and_cross_layer_test.go`. S89 is the next free
id. It lands in that same section, reusing its herdr-fake vocabulary.

**Value.** The rule "a live pane means the lane is attended, whatever
the bound session did" stops being a pair of unit tests and becomes a
documented cross-layer promise, executable: if a future change lets
board call an attended lane dead again, or lets the refusal point at a
teardown while a pane is live, S89 fails `go test ./...`.

**RED.** Add row S89 to the "Cross-layer: herdr and frit disagree"
table in the matrix, in id order after S88 — scenario "bound session
gone, pane still attends", outcome and mechanism naming that the survey
reports — board and the discovery render behind `ready` — show the lane
attended, not dead, and the refusal names the pane and leads with
resume, since a live pane outranks the gone bound session.
Add the `@S89` scenario to
[features/cross-layer.feature](../../features/cross-layer.feature) with
its Given/When/Then, not `@pending`. Run `go test ./cmd/frit -run
'TestFeatures/S89:'`: strict mode reports the new steps undefined and
the subtest fails. `go test ./internal/scenario` is green once the row
and the tag match. Commit the red.

The scenario, in the matrix's terms. Given a held lane whose bound
session herdr confirms gone, and a live *working* pane herdr shows on
that lane's branch: when `frit board --json` reports it, the lane is
not marked dead and the live pane is visible; when `frit ready --json`
lists it, it is not marked dead there either; and when an explicit
`frit start <id>` refuses it, the refusal names the pane and does not
lead with `frit yield`. The observables are the board, ready and start
documents, read through `--json` per the JSON Contract. Each assertion
branches on a field.

**GREEN.** Bind the scenario's steps in
`cmd/frit/bdd_identity_and_cross_layer_test.go`, reusing the herdr-fake
and lease vocabulary already there and defining only what S89 adds. A
step text another section's file or
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already defines
must not be redefined — strict mode reports it ambiguous. Every new
step function ships with its own unit test.

**Guard the edges.** `@S89` must be the scenario's only S tag, or the
bijection walk reports it. The scenario must assert both observables —
the board field and the refusal wording — so a regression in either
fails it. Confirm no other section's step file already binds a step
text this scenario introduces.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S89:'` passes with S89
PASS and none SKIP; `go test ./internal/scenario` stays green; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are clean;
`mdsmith check .` passes.

Write the handoff to `phase-3.result.md`: the S89 row wording as
landed, and any step vocabulary added to the section file.
