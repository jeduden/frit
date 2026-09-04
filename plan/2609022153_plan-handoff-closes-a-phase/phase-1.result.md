---
n: 1
title: The plan-handoff skill in the bundle
status: "✅"
result: true
summary: >-
  plan-handoff joined the bundle — internal/skills/assets/plan-handoff
  and its dogfooded .claude/skills copy — teaching the split close: a
  `## Handoff` heading for a single-file plan, a phase-N.result.md for
  a directory plan, either way riding the commit that lands the work.
---
## Handoff

`internal/skills/assets/plan-handoff/SKILL.md` is written and named in
`docs/development.md`'s skill-suite list. It splits the close on
`plans.IsFolderPlanFile`'s two shapes exactly as the phase spec asked:
a single-file plan gets its `## Handoff` heading and a `phases:` flip;
a directory plan gets `phase-N.result.md` with `{n, title, status,
result, summary}` front matter and the same heading, and flips its
sibling `phase-N.md`. Both close with the safe-to-clear cue.

No new machinery was needed in `internal/skills/skills.go` — it
already discovers skills by walking `assets/`, so adding the directory
was enough for `TestDogfoodCopiesMatchCanonical` to demand the
dogfood copy. That demand was the RED: the test failed on the missing
`.claude/skills/plan-handoff/SKILL.md` until `frit skills --via "go
run ./cmd/frit" --force` regenerated it. This phase's own close writes
that same `phase-N.result.md` shape it teaches.

Verified: `frit skills --via "go run ./cmd/frit"` into a scratch repo
lays down `plan-handoff/SKILL.md` with `{{frit}}` substituted;
`mdsmith check .` is clean on the touched files (the two `PLAN.md`
MDS019 failures predate this phase — confirmed by stashing and
rechecking — and are untouched); `go test ./...` and `golangci-lint
run` are clean.

**What's still open.** Tasks 2–4 — resume reading a single-file plan's
`## Handoff`, a doctor check for a skipped handoff, and pointing
plan-phase's step 4 at `/plan-handoff` — have no phase files yet; the
plan stays 🔳. The next phase should start by drafting phase-2.md for
task 2, since resume teaching the readback is what makes the skill's
single-file path actually observable end to end.
