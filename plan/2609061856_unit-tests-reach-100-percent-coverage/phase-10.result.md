---
n: 10
title: cmd/frit/start.go reaches 100% of its reachable lines
status: "✅"
result: true
summary: >-
  cmd/frit/start.go reached 100% of its reachable lines — twenty new
  tests, one dead branch deleted, and one listed exclusion — the
  densest of the five cmd/frit files.
---
## Handoff

`go test ./cmd/frit -coverprofile` started this phase with `start.go`
at roughly 90% of statements across ten partial functions. Every gap
closed as the phase spec expected, with two corrections the actual
code surfaced:

- **`livePaneOn`'s `p.Host != ""` skip was dead code, not an
  undertested branch.** `rt.herdr` is always wired to the local
  socket runner (`cmd/frit/main.go:3133`); `herdr.List` calls
  `ParseAgentList`, whose wire-to-`Pane` conversion never sets `Host`
  at all — only `herdr.ListHosts`'s own per-host fan-out tags it,
  after the fact, on results this call path never sees. A pane
  reaching `livePaneOn` can never carry a non-empty `Host`, so the
  skip could never fire. Deleted, the same treatment [phase
  8](phase-8.md) and [phase 11](phase-11.md) gave their own dead
  branches. `herdr.LiveRoots`, which the removed comment cited, is
  not the same case — it takes its panes from a caller that can
  genuinely merge in a remote host's own answers, so its own guard
  stays live; that comment is now corrected in place.
- **The unwind's own release failure renders as a lost-race refusal,
  not a raw command error.** `buildStart`'s `lostRace(err)` check
  matches a `FenceError` through `errors.Join`, so a handoff failure
  joined with a release-CAS failure still classifies as a lost race —
  exit 0, "refused", naming the new holder — rather than surfacing
  both causes as the phase's own proposed assertion expected. The line
  this phase targets (`startExecute`'s `releaseLease` call, 769-771)
  still runs and its error still matters; the test's assertions were
  corrected to match what the joined error actually renders as, not
  what it was assumed to render as.

Everything else closed exactly as the phase spec proposed:

- `Run`'s two early returns (`resolveSelector`, `gatherFleet`), and
  `buildStart`/`startResume`'s shared `!coordOK` gap, closed with the
  same CLI fixtures phases 7-9 already established.
- `unparkedSuffix` closed with a direct call against a repo with no
  local `plan/<id>` ref.
- `startExecute`'s `openEditor` error closed by overriding the
  existing seam.
- `agentStartPause`'s own unmocked body closed with one direct,
  real-sleep call — the sole addition to this suite's wall-clock time.
- `reconcileLeftoverWorktree`'s three gaps (`gitwt.List`, the park,
  the worktree remove) each closed with a direct call, the git-list
  and remove failures via a stub `runtime{git: func(...)}`, the park
  failure via a broken origin remote.
- `laneStandUpPane`'s fresh-`worktree.create` failure and
  `standUpLane`'s `herdr.Focus` failure both closed end to end through
  the CLI, each herdr fake erroring on exactly one verb.
- `editInEditor`'s five reachable branches all closed with the
  `$PATH`-resolution trick [phase 6](phase-6.md) proved for herdr,
  reused here for a throwaway editor script — default-to-`vi`,
  whitespace-only `$EDITOR`, an unwritable `$TMPDIR`, a non-zero exit,
  and a script that deletes its own argument file.

`go tool cover -func` filtered to `start.go` now reads 100.0% on
every function but `editInEditor`, at 88.9% — exactly the declared
1226-1233 exclusion, added to `scripts/coverage-exclude.txt`:
`WriteString`/`Close` on an already-open temp file, where POSIX
permission checks already ran at open time, so no directory or file
permission trick reaches a later write or close failure. `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are both
green. No BDD scenario was needed — every closed gap is an existing
path fed an input its tests never constructed, and the one deletion
is behavior-preserving.

`./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call — `main.go` and `progress.go` still carry gaps.

**Inherited by phase 11:** `main.go`'s own comment on `herdr.LiveRoots`
already reads correctly; check whether `localPanes(rt)` (the panes
`main.go:378`'s own `LiveRoots` call reads) can ever carry a
host-tagged pane either — if not, that call site may be carrying the
same dead-branch shape this phase just found in `livePaneOn`, worth
checking before assuming it needs a test rather than a deletion.
