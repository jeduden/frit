---
n: 2
title: The resume-path and window rows of host death and races run for real
status: "🔲"
result: false
---
Convert S14, S15, S18 and S31 from `@pending` into passing scenarios —
rows that exercise a verb, not the lease API alone. Phase 1 proved
the git-level CAS matrix; this phase proves what rides on top: the
token-based self-resume, the staleness window's unaided takeover, the
live-session veto, and the orphan report's silence before a window
matures. S32 is deferred: its mechanism needs a stateful herdr fake —
reflecting a first `start` call's own fresh worktree as live to a
second — that no existing fixture builds. These four each mirror an
existing unit test almost verbatim; a later phase covers S32,
alongside S29.

**Assumes.** The resume path, the window and the herdr fake, in brief:

- `claimCmd.Run` ([cmd/frit/claim.go](../../cmd/frit/claim.go)) calls
  `resumeOwnLease` before the ordinary readiness check. That is
  `resumeToken` plus persisting the beat: a persisted token
  (`claim.ReadToken`) matching origin's fresh tip, or
  `claim.OwnAdvance` covering a raw commit past the token (S86),
  gated by `herdr.SessionLive` on the marker's bound session. It also
  requires the calling directory to be this exact plan's own worktree
  (`inOwnLane`), so a resuming step must `t.Chdir` into the lane.
- A lost proof or a live session falls through to the ordinary path,
  which for an already-held plan refuses via `claimRefusal`:
  `"already held (<label>); <notMaturedReason>"` — the "refused" and
  "already held" wording S18 and S31 both key on.
- The staleness window is on-disk state, keyed by
  `observe.Key(repo, id)`. `seedWindow`
  ([claim_test.go](../../cmd/frit/claim_test.go)) backdates one
  directly — no clock seam, no sleep. A takeover fires once the span
  passes the repo's configured window (2h by default).
- `frit orphans`'s stale and deserted lists
  ([internal/report/orphans.go](../../internal/report/orphans.go))
  carry only matured holds and ones herdr positively confirms dead —
  "not among the panes herdr just listed." So an *unreachable* herdr,
  not an empty pane list, is what leaves a hold in neither list.
- `withHerdr` and `herdrReturning`
  ([cmd/frit/who_test.go](../../cmd/frit/who_test.go)) fake `agent
  list`. `startHerdr`
  ([cmd/frit/start_test.go](../../cmd/frit/start_test.go)) also
  answers `worktree create`, which a fresh mint or a takeover drives
  too; a resume never does.
- Existing unit tests already mirror each asserted fact: a lane's own
  resume over both an unlanded and a landed push; a matured window's
  unaided takeover, and its refusal short of maturity; a live agent
  vetoing that lane's own re-claim; a live bound session vetoing a
  takeover, with a beat pushed on the holder's behalf instead. The
  S18 fixtures live in `start_test.go` against `start --go`; rework
  them onto `claim`, the verb S18 names and `resumeOwnLease` guards.

**Value.** These four promises are what an operator actually leans on
when a host is gone. Coming back never needs a human step (S14). A
host that never comes back stops blocking the plan on its own clock
(S15). A zombie process cannot resume over a session someone else is
legitimately running in the same lane (S18). A report never claims a
merely-quiet host is dead, and its waking session is never steamrolled
by a takeover in flight (S31). A regression in any of them — a resume
that skips the veto, a takeover that fires early, an orphan report
that guesses — fails the build instead of surfacing in use.

**RED.** Drop `@pending` from S14, S15 and S18 in
[host-death.feature](../../features/host-death.feature) and from S31
in [races.feature](../../features/races.feature), and write each
one's Given/When/Then. Run `go test ./cmd/frit -run TestFeatures -v`
(the sanitized subtest name carries a colon after the id, not the
underscore this plan's own gate text assumed — phase 1's handoff
already flagged this, and verified RED and GREEN the same way). The
four new subtests fail on undefined steps. That is the red — commit
it.

The scenarios, in the matrix's own terms:

- S14, power loss mid-push. "This host" holds the lease, bound in its
  own lane; claiming again resumes from the persisted token — the
  push never landed. After a raw commit and push on that lane, claim
  again still resumes, now CASed from origin's fresh tip rather than
  the stale token — the push did land. Either way, no human step, no
  local damage past this lane.
- S15, host dies holding a claim, never back. "Elsewhere" holds the
  lease; once its window matures, this host claims and takes the
  lease over at epoch 2, a child of the stale tip. No human step.
- S18, zombie re-runs its own claim. "This host" holds the lease,
  bound in its own lane. With a live agent on that lane's session,
  claiming again is refused, naming the lease already held; origin is
  untouched. Once the agent goes quiet, claiming again resumes.
  RESUME only absent a live session on the lane, else VETO.
- S31, orphan report vs sleeping host. "Elsewhere" holds the lease,
  bound to a session herdr cannot confirm either way; the orphan
  report lists it as neither stale nor deserted — report only. Once
  the window matures and that session wakes and answers live, a claim
  is refused, naming the lease already held, and "elsewhere"'s lease
  is renewed by a beat instead of seized — VETO if host wakes.

**GREEN.** Extend `cmd/frit/bdd_host_death_and_races_test.go` — not a
new file. New section state and steps:

- `cliState` (via `section[cliState](w)`): the lane path and
  bound-session id the "bound in its own lane" Given records; the
  token tip persisted at setup, which S14's and S18's successful
  resume compare against; the raw tip a mid-scenario push leaves;
  whether a herdr fake was set explicitly yet; the last CLI run's
  stdout, stderr and exit code.
- `"this host" holds the lease for plan N, bound in its own lane` —
  `claim.Acquire` as `hostname()`, a fresh lane, a chosen session id
  (e.g. `wOld:p1`, matching no canned herdr session), `git worktree
  add` onto `plan/N`, `claim.Renew` to persist the token, `t.Chdir`
  into the lane.
- `"this host" commits raw work on its own lane and pushes it` — a
  raw commit, `git push origin plan/N`, recording the pushed tip.
- `this host claims plan N` — runs `frit claim N --root <root>`,
  `root` derived from the setup clone's parent directory. Absent an
  explicit herdr fake already set this scenario, installs
  `startHerdr()`'s runner as the default: S15's takeover needs
  `worktree create` answered; a resume ignores it either way.
- `a live agent sits on that lane's own session` / `the live agent
  goes quiet` — `withHerdr` a fake naming the setup step's own
  session live, or `herdrReturning()` with none.
- `"([^"]+)"'s bound session wakes and answers live` — S31's own
  live-session fake, mirroring the veto unit test's.
- `origin's orphan report lists plan N as neither stale nor deserted`
  — runs `frit orphans --root <root> --json`, unmarshals into
  `report.OrphansDoc`, asserts the plan id absent from both its stale
  and deserted lists.
- The remaining Then steps read the CLI's captured output and
  origin's marker chain, as the mirrored unit tests do: a takeover's
  epoch and parent, a refusal's wording, origin's hold left untouched,
  a resume's parent tip, a vetoed takeover's beat.
- `the hold's takeover window has matured` reuses `seedWindow`
  directly. `"([^"]+)" holds the lease for plan (\d+)$` reuses
  `bdd_lease_test.go`'s step for S15's and S31's "elsewhere" setup.

**Guard the edges.** Three traps:

- `SessionDeadIn` reads any session absent from the panes herdr just
  listed as dead. An empty, reachable `herdrReturning()` is not
  "unknown" — it is "confirmed gone." S31's first half must leave
  herdr genuinely unreachable instead (an error-returning closure), or
  "neither stale nor deserted" fails against the mechanism it proves.
- S18's refusal wording is `claimCmd`'s own `claimRefusal`, distinct
  from `start`'s `liveHoldRefusal` — the mirrored fixtures are
  reworked onto `claim`, not copied verbatim.
- A step text already defined must not be redefined.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(14|15|18|31):'`
passes with every one of the four reported PASS and none SKIP — the
colon, not phase 1's plan-wide gate text's underscore. `go test
./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`. Confirm or correct the
`SessionDeadIn` reading above against the actual test run. Say what
S32 and S29 still need, refined by whatever this phase's steps
happened to touch along the way.
