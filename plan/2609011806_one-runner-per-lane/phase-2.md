---
n: 2
title: The skills treat a dispatched phase as already running
status: "🔲"
result: false
---
Close the vector frit cannot intercept. A re-typed `/plan-phase` is not
a frit verb, so the skill is the only guard. Teach
[plan-pick](../../internal/skills/assets/plan-pick/SKILL.md) that
`pick --go` dispatches the phase into a fresh pane. A result carrying
`prompt_dispatched: true` means the phase is already running there.
Report the pane, and never run `/plan-phase` yourself — a second run
puts two runners on one lane. Add the mirror note to
[plan-phase](../../internal/skills/assets/plan-phase/SKILL.md): a lane
already dispatched is already running; start no second runner on it.

**Assumes.** `prompt_dispatched` is a real `--json` field, true exactly
when the prompt was sent into a pane (Phase 1 left it untouched). The
canonical skill text lives in `internal/skills/assets`; `frit skills`
regenerates frit's own `.claude/skills` with `--via "go run ./cmd/frit"`,
and `TestDogfoodCopiesMatchCanonical` in
[internal/skills/skills_test.go](../../internal/skills/skills_test.go)
fails if a dogfood copy diverges. The `skill` kind in
[.mdsmith.yml](../../.mdsmith.yml) caps each skill at 650 heuristic
tokens, so the new guidance must earn its words.

**Value.** A caller reading `pick --go`'s `--json` learns the phase is
already running and stops, instead of re-running it and racing the pane
it just dispatched. This is the fix for the vector the issue leads with —
the one no frit-side guard can reach, since the re-run never passes
through a frit verb.

**RED / GREEN.** A skill phase gates on the claim against the built
binary, not on lint. `TestDogfoodCopiesMatchCanonical` and the token
budget both pass on a false claim. Edit the canonical
`plan-pick/SKILL.md` and `plan-phase/SKILL.md` under
`internal/skills/assets`, each kept within its token budget. Then run
`go run ./cmd/frit skills --via "go run ./cmd/frit" --force` to
regenerate frit's own copies, so the dogfood-match test stays green.

**Gate.** `frit pick --go --json` and `frit start --json` report
`prompt_dispatched` exactly as the skill now describes it — false on a
dry run, true when the prompt was dispatched. `go test ./...` (with
`TestDogfoodCopiesMatchCanonical`), `go tool -modfile=tools/go.mod
golangci-lint run`, and `mdsmith check .` are all green.
