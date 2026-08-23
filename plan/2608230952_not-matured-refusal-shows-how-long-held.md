---
id: 2608230952
title: A not-matured hold refusal shows how long it has been held
status: "🔳"
summary: >-
  When frit refuses to act on a held lane because its takeover window
  has not matured, it says only "held by a live lease" — not how long
  the tip has sat unchanged nor how far that is from the window. The
  observer already computes StaleFor; surface it against the repo's
  configured window so the operator knows how close the hold is to
  being takeable.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: reap's not-matured refusal reports StaleFor against the window
    status: "🔲"
  - n: 2
    title: claim and start refusals carry the same span, and a voided window says why
    status: "🔲"
---
# A not-matured hold refusal shows how long it has been held

## Goal

frit sometimes refuses to act on a held plan because its takeover
window has not matured. The refusal should report two things: how long
the tip has been observed unchanged, and the window it is measured
against. "Held, seen unchanged for 42m of the 2h takeover window" —
instead of a bare "held by a live lease". The operator can then judge
how close the hold is to being takeable.

## Context

The datum already exists. `discovery.Plan.StaleFor`
([internal/discovery/discovery.go](../internal/discovery/discovery.go),
~line 64) is "how long the tip has been observed unchanged". The
staleness observer sets it. The refusal that hides it is `holdRefusal`
([cmd/frit/reap.go](../cmd/frit/reap.go), ~line 323). Its
`!p.Stale && !p.Dead` case returns a fixed string — "held by a live
lease; reap drops only a stale or dead hold — its own lane releases it,
or claim takes it over once the window matures". It reads neither
`p.StaleFor` nor the window.

Reuse searched:

- **`p.StaleFor`** — already carried on every gathered `discovery.Plan`;
  the refusal just never reads it. No new observation work.
- **`staleClock(res, repo)`**
  ([cmd/frit/main.go](../cmd/frit/main.go), ~line 1102) — already
  returns the repository's configured takeover window T and sample gap
  S_max, honoring `.frit.yml`'s `takeover-window`. Reused so the
  message names the repo's real window, not a hardcoded 2h.
- **`holdRefusal`** ([cmd/frit/reap.go](../cmd/frit/reap.go)) — the one
  function that words this refusal; enriching it in place keeps the
  wording in a single spot. It currently takes `(p, ok)`; the window T
  must reach it (thread the value `staleClock` already resolves in
  reap's Run through to the refusal).
- **`discovery.Window.Voided`**
  ([internal/discovery/stale.go](../internal/discovery/stale.go),
  ~line 31) — records why a window was thrown away ("window restarted:
  a 45m gap exceeded the 30m bound"). It is NOT on `discovery.Plan`
  today — only `StaleFor` is — so surfacing "why it has not accrued"
  needs new plumbing and is deferred to Phase 2.

The window is already configurable per repo. The enriched message
reflects the repository's own `takeover-window`. This plan does not
touch the config path.

## Tasks

1. reap's not-matured refusal reports StaleFor against the configured
   takeover window, proven by a message assertion.
2. (determined after Phase 1)

## Phase 1: reap's not-matured refusal reports StaleFor against the window

**RED.** In [cmd/frit/reap_test.go](../cmd/frit/reap_test.go), add a
test that runs `reap` over a fleet with one held-but-unmatured plan
(reuse the existing live-lease refusal fixture) whose observed window
has a known `StaleFor` short of the window T. Assert the refused-hold
reason names the span and the window — it contains the StaleFor
duration and the takeover window (e.g. matches both "unchanged for" and
the window value) — not just "held by a live lease". It fails today:
`holdRefusal` emits the fixed string.

**GREEN.** Thread the takeover window T (from the `staleClock` value
reap's `Run` already has, or resolve it there) into `holdRefusal`, and
in the `!p.Stale && !p.Dead` case format the reason with `p.StaleFor`
and T: "held by a live lease, seen unchanged for <StaleFor> of the <T>
takeover window; not takeable until the window matures". Keep the
existing recovery clause. `holdRefusal`'s other case (no observed lease
state) is unchanged.

**Gate.** `go test ./cmd/frit -run TestReap` green including the new
assertion; `go build ./...`; `go vet ./...`; the JSON report golden is
re-recorded only if a refusal reason field feeds it —
`go test ./internal/report` green.

## Phase 2: claim and start share the span; a voided window says why

**RED.** Add tests that `claim <id>` and `start <id>` on the same
held-but-unmatured plan report the same span-and-window phrasing in
their refusal. Reuse each verb's existing not-startable refusal test.
Add one more test: when the plan's window was voided by a sample gap,
the refusal also carries that reason ("window restarted: a … gap
exceeded the … bound"), so a hold whose span keeps resetting explains
itself. They fail today. Those refusals still emit the bare wording,
and `Voided` is not on `discovery.Plan`.

**GREEN.** Word the span-and-window reason once in a shared helper and
call it from `claim`'s and `start`'s not-matured refusals as well as
reap's. Thread `discovery.Window.Voided` onto `discovery.Plan` through
the same observe/gather path that already sets `StaleFor`, and include
it in the reason when present.

**Gate.** `go test ./cmd/frit -run 'TestReap|TestClaim|TestStart'`
green; all three verbs speak the same span-and-window reason; a voided
window is explained; `go build ./...`, `go vet ./...`,
`go test ./...` all green.

## Execution

| Phase | Work                                                                       | Tier   |
| ----- | -------------------------------------------------------------------------- | ------ |
| 1     | Proving slice: reap's not-matured refusal shows StaleFor vs the window     | sonnet |
| 2     | Share the wording across claim/start, and surface a voided window's reason | sonnet |

## Acceptance Criteria

- [ ] A not-matured hold refusal names how long the tip has sat
      unchanged (StaleFor) and the repo's configured takeover window.
- [ ] The window shown honors `.frit.yml`'s `takeover-window`, not a
      hardcoded value.
- [ ] reap, claim, and start all speak the same span-and-window reason.
- [ ] A hold whose window was voided by a sample gap carries that
      reason, so a span that keeps resetting explains itself.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...`, and
      `mdsmith check .` all pass.
