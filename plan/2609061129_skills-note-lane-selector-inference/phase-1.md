---
n: 1
title: plan-phase names the in-lane inference
status: "✅"
result: false
---
Give plan-phase the one fact it withholds: run in the plan's own lane,
a verb infers the id from the branch, so the selector can be omitted.
The note lands in the canonical asset and its regenerated dogfood copy.
It proves the wording, the token budget and the real-lane gate the
later skills copy.

**BDD coverage.** None applies. This phase ships skill text, not a
command behavior; its gate is the built `frit` run in a real lane, not
a scenario.

**Assumes.** The canonical text is plan-phase's asset under
[internal/skills/assets](../../internal/skills/assets), whose commands
read `{{frit}}`. `frit skills --via "go run ./cmd/frit"` regenerates
the dogfood copy under [.claude/skills](../../.claude/skills), guarded
by `TestDogfoodCopiesMatchCanonical` in
[internal/skills/skills_test.go](../../internal/skills/skills_test.go).
The `skill` kind caps the file at 650 heuristic tokens. plan-phase's
Inputs already reads "Plan id, or enough of the title to resolve it".

**Value.** The surface an in-lane agent loads finally carries the fact
the reference docs already hold. The agent that did not know it could
omit the id now reads it where it works.

**RED.** No test turns red first here — the change is skill prose, and
the gate is a real-lane run, not an assertion. Guard against a false
green instead: before editing, confirm `TestDogfoodCopiesMatchCanonical`
is green, so a later divergence between asset and copy is caught.

**GREEN.** Add one terse clause to plan-phase's Inputs (or the Load
step), in the canonical asset: in the plan's own lane the id is
inferred from the branch, so `{{frit}} phase` needs no selector; pass
one only to act on another plan. Keep it a clause, not a new section.
Regenerate the dogfood copy with `frit skills --via "go run
./cmd/frit"`; never hand-edit the copy.

**Guard the edges.** The note must not push plan-phase over its
650-token budget — trim elsewhere only if `mdsmith check` fails on
MDS028, never loosen `.mdsmith.yml`. Reuse the word the skills already
use for a plan ("its own lane"), so the vocabulary stays one.

**Gate.** In a scratch fleet, from inside a lane's worktree, the built
`frit phase` with no selector resolves that lane's plan — the claim
the new line makes, confirmed against the binary, not lint.
`TestDogfoodCopiesMatchCanonical` and `mdsmith check
internal/skills/assets/plan-phase .claude/skills/plan-phase` are green.
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green.
