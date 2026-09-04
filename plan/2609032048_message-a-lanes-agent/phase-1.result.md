---
n: 1
title: frit message sends an operator's text to a live lane
status: "✅"
result: true
summary: >-
  `frit message <id> "<text>"` sends arbitrary text to a plan's live
  lane through `herdr.Prompt`, working or idle — the one deliberate
  divergence from `nudge`, which refuses a busy lane. Dry-run by
  default, `--go` to send, reporting a sibling `report.MessageDoc`
  built the same way `NudgeDoc` is. The skill front rides the same
  change.
---
## Handoff

`messageCmd` sits beside `nudgeCmd` in `cmd/frit/dispatch.go`, resolving
the selector, finding the lane through the same `liveLaneFor`, and
carrying the same herdr-unreachable and presence-unknown handling
verbatim from `nudge`. `messageSend` is `nudgeSend` with its idle gate
removed — the one deliberate difference the phase spec called for — so a
working lane is targeted and sent to exactly like an idle one. Under
`--go` it calls `herdr.Prompt` with the operator's text unmodified;
without it, it dry-runs and only prints the target and text.

The report chose a **sibling doc**, `report.MessageDoc`
(`internal/report/dispatch.go`), not an extended `NudgeDoc`: message
carries no phase, no tier and no composed prompt — the fields that make
up half of `NudgeDoc` — so bolting `Text` onto it would leave those
empty on every message and readers guessing which fields apply. The
sibling carries exactly `root`, `plan`, `text`, `target`, `go`, `sent`,
`refused`, `problems`, pinned in `internal/report/testdata/message.json`
via `goldenMessage`.

Verb and flags: `frit message <selector> <text> [--go]`. Both `selector`
and `text` are required positional arguments — unlike `nudge`'s
optional, cwd-inferring selector, kong refuses a required positional
after an optional one, and `text` must always follow `selector`, so
`message` never infers from the cwd.

**Verified.** `go run ./cmd/frit message 2609032048 "are you in a PR?"`
against this very lane's own live pane (`w56:p1`, `status: working`)
named the pane and showed the text under dry-run without refusing for
being busy — the gate's own working-lane case, proven against a real
herdr rather than only the fake. `go run ./cmd/frit message --help`
reads as a peer of `nudge --help`. The `plan-drive` skill gained an "Ask
directly" section naming `message` for a held lane whose state is
unclear; `frit skills . --force --via "go run ./cmd/frit"` regenerated
the dogfood copy and `TestDogfoodCopiesMatchCanonical` passes.
`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are all clean.

**Code review addendum.** A recall-biased review of this phase found
`messageSend` sent to any found lane regardless of `Pane.Presence()`,
including the honest-unknown status herdr reports for a pane it cannot
read — a gap `nudgeSend`'s idle-only gate never had, and one the
acceptance criteria's own "working or idle" framing did not intend to
cover. `messageSend` now refuses `herdr.StatusUnknown` the way
`nudgeSend` refuses anything but idle, and `messageCmd.Run` now refuses
an empty `Text` outright rather than dry-running (or, under `--go`,
silently sending) nothing — mirroring the empty-`Selector` guard already
next to it. `TestMessageRefusesAnUnknownStatusLane` and
`TestMessageRefusesEmptyText` pin both. `go test ./...`, the linter and
`mdsmith check .` are clean after the fix.

**What Phase 2 inherits.** `liveLaneFor` is the one join Phase 2 should
reuse to reach "a live pane attends" at the deserted-refusal and survey
sites — it already resolves a plan's hold branches to the live lane in
its own repository, the exact fact those refusals need before naming
`frit message <id>` as the remedy instead of `frit yield` or a
hand-land. No new helper was introduced here; `liveLaneFor` and
`presenceUnknown` are unchanged and carry directly. The plan stays 🔳:
Phase 2 (routing) and Phase 3 (S90 under godog) have no phase files
written yet.
