---
n: 2
title: The resume-path and window rows of host death and races run for real
status: "✅"
result: true
summary: >-
  S14, S15, S18 and S31 drop `@pending` and pass as real godog
  scenarios driven through the CLI (`frit claim`, `frit orphans`)
  rather than the lease API alone, each mirroring an existing
  claim/start unit test's fixture. A new `cliState` section carries
  the lane, bound session, persisted token and captured CLI output a
  verb-level row needs, alongside the shared `world`.
---
## Handoff

**`SessionDeadIn` reading confirmed.** The spec's caution held: an
empty, reachable `herdrReturning()` reads every bound session as
confirmed dead, not unknown, so S31's report-only half installs a
genuinely unreachable herdr fake (an error-returning closure) rather
than an empty one. Verified by the scenario passing — a bare empty
fake would have put the plan in `Deserted` and failed the "neither
stale nor deserted" assertion.

**One correction the implementation surfaced.** The phase spec wrote
S31's takeover refusal with S18's exact wording,
`"the claim is refused, naming the lease already held"`. That is
wrong: a *matured* window routes the attempt through
`mintOrTakeOver`'s own live-session veto (`vetoRefusal`, "is held by a
live agent session on `<holder>`; its lease was renewed on its
behalf"), never through `claimCmd`'s `claimRefusal` S18 actually hits
(fired only for a held, not-yet-matured plan). S31 got its own step
and wording, `"the takeover is refused, naming a live agent session"`,
both in the feature file and in `bdd_host_death_and_races_test.go`.
S18 itself needed no change — its window is never seeded, so
`resumeOwnLease`'s veto genuinely falls through to `claimRefusal`.

**Design decision: CLI-driven, not lease-API-driven.** Every step in
this phase runs `frit claim` or `frit orphans` through `run(...)`,
capturing stdout/stderr in the new `cliState` section, rather than
calling `internal/claim` directly the way phase 1's rows did. The
resume, the window's takeover and the live-session veto all live in
`cmd/frit/claim.go`, not in the lease package — mocking at the lease
level would have proven nothing about the actual promise. `cliState`
adds no field to the shared `world`; it rides `section[T]` exactly as
phase 1's `racesState` and `hostDeathState` do.

**The herdr-fake default.** `this host claims plan N` installs
`startHerdr()` as a default only when no herdr fake was set explicitly
yet this scenario (`cliState.herdrSet`). S15's takeover needs
`worktree create` answered; S14's and S18's resumes ignore it either
way, so the same default served all three. S18 and S31 each override
it explicitly before their own claim, and the override sticks — the
default never clobbers a scenario's own fake.

### What S32 and S29 still need

- S32, two same-host sessions race. The right mechanism is confirmed:
  `startLiveLaneRefusal` / `liveLaneRefusal` in `cmd/frit/start.go`,
  keyed on herdr's pane list matching the plan's own branch by `cwd`,
  independent of holder identity. No existing fixture drives two
  sequential `frit start --go` calls where the second sees the
  first's own freshly-created worktree as live — `startHerdr()`'s
  canned `agent list` is static and carries no `cwd` at all, so it
  cannot reflect a just-created worktree back. The row needs a
  stateful herdr fake: a closure that starts empty and, once
  `worktree create` is answered, adds a live pane at that path for
  every later `agent list` call.
- S29, release races a loser's read. Unchanged from phase 1's
  handoff: needs a `gitwt.Runner` wrapper around `gitwt.Exec` that
  injects a release push between a loser's failed CAS and its
  reconciliation read. Test scaffolding in the step file, not a seam
  added to any verb.

`go test ./...`, `go test ./internal/scenario` (the bijection gate),
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
are all clean.
