---
id: 2611092115
title: frit works in a Claude project's cloud session
status: "✅"
summary: >-
  Run inside a Claude project thread's cloud session, frit took the
  herdr-less path and pushed plan/<id> fine, but two things broke.
  Every cloud container is called vm, so every session held under one
  name. And the session's git proxy refuses pushes to refs/frit/* and
  branch deletes, so a park, a yield and a reap fail with git's bare
  403. A cloud session now holds as its own session id, and a park
  writes a branch under frit/rescue/ that the proxy admits.
model: sonnet
depends-on: [2610132034]
---
# frit works in a Claude project's cloud session

## Goal

A Claude project thread, running in its own cloud session, can claim,
work, publish and release a plan with frit. Parallel threads never
read as one host.

## Context

**What was tried.** frit was built and run in a Claude project
thread's cloud session against this repository's origin. herdr is not
installed there, so the herdr-less path of plan 2610132034 is taken.
The board, `who`, `doctor` and a dry-run `reap` all ran clean.

**What the git proxy allows.** Plan 2610132034 phase 4 left open
whether a cloud session's git proxy lets it push `plan/<id>` at all.
It does. A probe branch `plan/9999999999-proxy-probe` was created
there. Two pushes were refused with HTTP 403:

- deleting that branch;
- creating a ref under `refs/frit/rescue/`.

Force-pushes were not tried. Every lease transition but one is
append-only — claim, beat, release and takeover each mint a child of
the tip — so it never needs one. The exception is a scavenge's delete.

**What breaks.** Two things:

- *One hostname for every session.* `os.Hostname` answers `vm` in
  every Claude Code cloud container. A marker names its holder by
  hostname, so parallel threads, the shape a coordinator spawns, all
  hold as `vm`. A race lost to another thread reads "already held on
  this host (vm)". A checkout standing on another session's lane
  passes the preflight that refuses a foreign hold, and the verb then
  resumes that lease as its own.
- *Refused refs.* A park pushes to `refs/frit/rescue/...`, and a
  scavenge deletes the work ref. Both are refused, so `yield`,
  `reap --go` and a start that parks a leftover fail with git's bare
  403 error.

**The session's identity.** The container carries its session id in
`$CLAUDE_CODE_REMOTE_SESSION_ID`, unique per session. frit already
resolves its own settings through the four-place chain, so the holder
becomes one more: `--holder`, `$FRIT_HOLDER`, or `holder:` in a config
file. With none set, a cloud session holds as its session id, and any
other host by its hostname, as before. The index's host column stays
the hostname: it keys plans, not leases.

## Tasks

1. Phase 1: a cloud session holds as its session id, and `--holder`
   names any host's holder through the settings chain.
2. Phase 2: a park writes a branch under `frit/rescue/`, which the
   proxy admits. Owner's choice, over refusing teardown in a cloud
   session. Old `refs/frit/rescue/` refs are still read, and the
   index skips rescue branches. A delete stays refused there, so
   `reap --go` belongs on a host that can delete.

## Execution

| Phase | Title                                       | Tier   | Gate                                                                                                                  |
| ----- | ------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------- |
| 1     | a cloud session holds as its own session id | sonnet | C26 on the built frit: the marker names the session, and a second session's verb from that lane refuses it as foreign |
| 2     | teardown answers the cloud proxy's refusals | sonnet | decided with the owner; a scenario whose remote refuses `refs/frit/*` and deletes                                     |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                            |
| --- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [a cloud session holds as its own session id](phase-1.md)                                                                                                                                                                        |
|     | ↳      | A cloud session now holds as its session id, and --holder names any host's holder through the settings chain. A second session run from the first one's lane refuses it as foreign instead of resuming it. C26 covers it.        |
| 2   | ✅     | [a park lands where the cloud proxy admits it](phase-2.md)                                                                                                                                                                       |
|     | ↳      | A park now writes a branch under frit/rescue/, which a cloud session's proxy admits, and old refs/frit/rescue/ refs still list. The index skips rescue branches. C27 parks through a remote that refuses what the proxy refuses. |
<?/catalog?>

## Acceptance Criteria

- [x] In a cloud session with no setting, a claim's marker names the
      session id, not `vm`
- [x] `--holder`, `$FRIT_HOLDER` or `holder:` in a config file names
      the holder on any host
- [x] A verb run from a lane another session holds refuses it as held
      by that session, rather than resuming it
- [x] Through a remote that refuses refs outside `refs/heads/` and
      every delete, `yield` parks a fenced lane and tears it down
- [x] Rescues parked under `refs/frit/rescue/` before this still list
- [x] The index never reads a plan off a rescue branch
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
