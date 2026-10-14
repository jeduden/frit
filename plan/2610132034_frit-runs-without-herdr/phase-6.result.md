---
n: 6
title: a host with herdr installed can opt out with --no-herdr
status: "✅"
result: true
summary: >-
  --no-herdr, FRIT_HERDR=false or herdr: false in a config file runs a
  host with herdr installed exactly as one without it, never running
  the local binary; nudge and message name herdr turned off. C24
  proves it.
---
# Phase 6 result

## Handoff

A host with herdr installed can now opt out. `--herdr` is a global,
negatable setting, on by default and shaped like `--fetch`. It
resolves from the flag, `$FRIT_HERDR`, or `herdr: false` in a config
file. When it is off, the runtime's herdr runner becomes `herdr.Off`,
which runs nothing and answers `herdr turned off (--no-herdr)`. The
not-found check reads that answer as absence, so every herdr-less
path from phases 1–5 applies unchanged: git lanes, hand-backs, the
git teardown, and the parked leftover. nudge and message name the
reason, either `herdr not installed` or `herdr turned off
(--no-herdr)`. Configured remote hosts are still read over ssh,
because the setting is this host's.

**Verified by.** Unit tests, written failing first, for the off
runner and the not-found check. They also cover a claim opted out by
flag, by environment and by config file, each standing a git lane up
while a recording herdr sees no call, and the refusal wording. C24
runs through godog against the built frit with a herdr on `$PATH`
that logs every call. `start 7 --go --json --no-herdr` hands back in a
git lane, and the log stays empty. With the opt-out unwired, C24
fails on the fake herdr's own `worktree create`.
