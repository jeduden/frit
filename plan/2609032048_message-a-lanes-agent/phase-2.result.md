---
n: 2
title: the ambiguous-lane output routes to message
status: "✅"
result: true
summary: >-
  The deserted refusal an attended lane meets now leads with `frit
  message <id> "what is your status?"` ahead of resume and yield, and
  the survey reports carry the same pointer machine-readably: a new
  `ask` field on the discovery card and the board row, set only for a
  held lane whose bound session is gone but whose branch a live pane
  attends. The board table prints that ask beneath its rows. A no-pane
  lane and a bound live lane are untouched.
---
## Handoff

**Where the remedy is emitted — both.** The refusal and the survey
reports each name the ask, built from one text so they cannot drift:
`report.AskCommand(id)` in `internal/report/dispatch.go`, beside
`MessageDoc`, renders `frit message <id> "what is your status?"`. The
question rides along because `message` takes its text as a required
positional — a bare `frit message <id>` would refuse — so the reader
runs it verbatim.

**The refusal.** `resumeRefusal` in `cmd/frit/start.go` — what
`desertedRefusal` and `parkFirstRefusal` both render once `liveLaneFor`
finds a pane on the branch — now reads: "deserted hold: a live herdr
pane (<pane>) on lane <branch> attends it; ask it with `frit message
<id> "what is your status?"` or resume it with `frit open <id>` — run
`frit yield <id>` only to set the work aside instead". Ask leads, resume
follows, yield trails — so S89's pin (open before yield) still holds
and `TestResumeRefusalNamesMessageAsTheWayToAsk` pins message before
yield. Both `start` and `claim` reach it through the same two refusals;
no site was added, so a no-pane deserted lane still names yield alone.

**The survey reports — a field, not a rendered hint.** Against the JSON
Contract the choice was a machine-readable pointer: `ask` (string,
always present, `""` when there is nothing to ask) on
`report.PlanCard` — set at the shared `cardsOf` site so `ready`, `pick`
and `find` cannot drift — and on `report.BoardPlan`, set in
`BoardDoc.AddPlan`. Both go through one gate, `askOf(p, attended)` in
`internal/report/discovery.go`, whose inputs are exactly the deserted
reading's own: `Held && Dead && !Stale`, plus the live-pane fact. That
fact is the same one 2609031939 already threads in to clear `Dead` — the
`attended` callback for the cards, `agent != ""` for the board — so no
new herdr read was added. A card built straight from `cardOf` (`next`'s
Plan) carries `ask: ""`, as it carries `Dead` unreconciled. The golden
files for ready, pick, find, board, next, next-lane and phase gained the
key with an empty value; no existing key moved.

**The board table.** `boardAsks` in `cmd/frit/main.go` prints one line
per row with a non-empty ask, after the legend, whatever `--columns`
shows — it keys no marker, so unlike the legend it is not tied to the
hold column: `<id>: its bound session is gone but <agent> still attends
it; ask before yielding: frit message <id> "what is your status?"`. The
ready, pick and find tables are unchanged, matching their existing
divergence of not rendering `dead` either; their JSON carries the field.

**Edges held.** A no-pane deserted lane refuses with yield alone, reads
`dead: true` with `ask: ""`, and its board legend renders as before
(`TestPrintBoardOffersNoAskForAnUnattendedDeadHold`). A bound live lane
— held, attended, not dead — gets no ask (`...ForABoundLiveLane` on
both the card and the board row). A matured (stale) hold is left to
`staleHeld`'s own cell, as `desertedRefusal` leaves it.

**The skill.** `plan-drive`'s "Ask directly" section now says
`board --json` names the case: a held row whose `ask` is non-empty
carries the exact command. The dogfood copy was regenerated with
`frit skills . --force --via "go run ./cmd/frit"`.

**What Phase 3 should reproduce end-to-end.** The exact S89 fixture is
the state: this machine holds plan 7 in a lane bound to a session with
its token persisted, a takeover bound to a session at a new epoch lands
on plan 7 (so herdr confirms the original session gone — `Dead`), and
herdr shows a live pane (`wLive:p1`, agent `claude`, status `working`)
on the lane. Then pin the whole route to "ask the agent":

- `frit board --json` — plan 7's row has `dead: false` and
  `ask` equal to `frit message 7 "what is your status?"`; the plain
  board prints that command beneath the table.
- `frit ready --json` — plan 7's card carries the same `ask`.
- `start --go` for plan 7 from the lane — refuses, and the refusal
  names `frit message 7` before `frit yield`.
- `frit message 7 "what is your status?"` against the same fake pane —
  dry-runs naming `wLive:p1`, and under `--go` the fake receives the
  text (herdr `Prompt` over the pane `liveLaneFor` finds) without a
  busy refusal, since the pane is `working`.

**Id collision to resolve first.** Plan 2609031951 landed its own
`@S90` ("a deserted top lane in pick's walk") after this plan was
written, so S90 is taken in `features/cross-layer.feature` and in the
matrix at `docs/research/lease-protocol.md`. Phase 3 should take the
next free id — S91 as the tree stands — and update its phase text, the
plan's Execution row and the acceptance criteria to match, rather than
overwrite 2609031951's row.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are all clean. The plan stays 🔳 for Phase 3.
