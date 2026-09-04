---
n: 1
title: no report calls an attended lane dead
status: "✅"
result: true
summary: >-
  board and the discovery card behind ready, pick and find no longer
  render `dead: true` for a held lane whose bound session herdr
  confirms gone but whose branch a live pane still attends, working or
  idle. The rendered field is gated on the live pane rather than
  copied straight from the identity fact, applied once at each
  render's shared site so ready, pick and find cannot drift.
---
## Handoff

Landed as scoped, on the smaller of the two field shapes the phase
spec offered.

**Field-shape decision.** Gated the rendered `dead` on the live pane
rather than adding a companion "attended" boolean. `dead: true` now
means exactly what a reader takes it to mean — "nobody is here" — and
every existing consumer of the field keeps working unchanged when no
pane attends. Adding a second field would have made two facts do the
work one already can, and the JSON Contract asks for the smaller
change that removes the contradiction, not a wider one. `agent` /
`agent_status` still carry the pane's own fact on board, so a consumer
that wants the raw identity fact under a live pane still has it there.
`discovery.Plan.Dead` itself is untouched — the takeover and refusal
decision logic still reads the unqualified identity fact, exactly as
the phase's "Two Deads, kept apart" section requires.

**The two reconciliation sites.** Board and the discovery card each
gate their own copy of the field:

- `internal/report/board.go`'s `AddPlan` now renders
  `Dead: p.Dead && agent == ""` — board already took `agent`/`status`
  as arguments, so no new parameter was needed. `agent == ""` is the
  same "nobody found" idiom `agentLabel` and the existing
  `case held: return "idle"` fallback already use elsewhere in board's
  own rendering, so this reuses established codebase convention rather
  than introducing a new one.
- `internal/report/discovery.go`'s `cardsOf` — the one function
  `ReadyDoc.SetPlans`, `PickDoc.SetPlans` and `FindDoc.SetPlans` all
  call — now takes an `attended func(discovery.Plan) bool` and clears
  `card.Dead` when it reports true for that plan. `cardOf` itself (the
  single-plan projection `phase` and `next`/`show` also use) is
  untouched, so those out-of-scope verbs render exactly as before —
  passing `attended: nil` through `cardsOf` is a no-op, which is also
  what a nil predicate does in every test call site that does not care
  about liveness. Reconciling once here, rather than per-doc, is what
  keeps `ready`, `pick` and `find` from drifting apart; all three are
  proven directly in `internal/report/discovery_test.go`.

**How the live-pane fact was plumbed to the agent-less reports.** The
discovery card carries no `agent` field to piggyback on, so `cardsOf`
needed a purpose-built predicate rather than board's `agent == ""`
reuse. `cmd/frit/main.go` gained `attendedFor(p discovery.Plan, live
map[string]herdr.Lane) bool` beside the existing `agentFor` — it walks
the same `p.Holds` branches and reports whether *any* lane is live
there, independent of whether that pane names an agent. `readyCmd`,
`pickCmd` and `findCmd`'s `Run` now call `liveByBranch(c, rt)` — the
exact herdr read `board.Run` already made — and pass
`func(p discovery.Plan) bool { return attendedFor(p, live) }` into
`SetPlans`. This is one `fleetPresence` read per command invocation,
the same one board already pays; no second query was added anywhere.
`hostProbs` from that read are carried into each doc's `Problems` the
same way board already does, so an unreadable host stays visible
there too.

One gap worth naming, corrected from an earlier draft of this note:
`AddPlan`'s `agent == ""` gate and `cardsOf`'s `attendedFor` gate do
not diverge from each other, because both read the same `live` map
`liveByBranch` builds from `whoLanes`, and `whoLanes` keeps only
panes with `HasAgent()` true before that map is ever built. A live
*bare* pane — no agent attached, a person sitting in a plain shell on
the branch — never becomes an entry in `live` at all, so neither gate
sees it: `dead` stays true through `AddPlan` and through `cardsOf`
alike. No test exercises a bare pane on a bound-session-gone hold, and
reaching one means widening what `whoLanes` keeps, not reconciling
`AddPlan` against `cardsOf` — the two already agree. Worth a look if a
bare-pane report ever surfaces the same contradiction this phase
fixed for an agent's pane.

**Guard the edges, confirmed.** `TestAddPlanStillMarksAnUnattendedDeadHoldDead`
and `TestReadySetPlansStillMarksAnUnattendedDeadLaneDead` pin that a
bound-session-gone hold with no live pane still renders `dead: true`
in both board and the discovery card, unchanged. `frit orphans` was
not touched — its `StaleHold.Dead` and `Deserted` category still read
straight from the identity fact, as the plan's "Two Deads, kept apart"
section calls for.

**For Phase 2.** The refusal site (`cmd/frit/start.go`'s
`desertedRefusal` and `parkFirstRefusal`, inside `buildStart`) already
runs `liveLaneFor` a few lines later for `liveLaneRefusal`'s own use —
that call's `found bool` return is the single-plan equivalent of this
phase's `attendedFor`: both ask "is any lane on one of this plan's
`Holds` branches live now", just at different fan-outs (one plan here
vs. board/ready/pick/find's fleet-wide `live` map there). Phase 2
should hoist `liveLaneFor`'s call earlier in `buildStart` and gate
`desertedRefusal`/`parkFirstRefusal` on its `found` result directly,
rather than introducing a second helper — there is no fleet-wide `live`
map to reuse at a single-plan refusal site, so `attendedFor` itself
does not apply there, only the same "found means attended" reading it
encodes.

Verified: `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are both clean.
