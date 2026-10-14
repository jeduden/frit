---
n: 6
title: a host with herdr installed can opt out with --no-herdr
status: "✅"
result: false
---
Phases 1–5 switch to the herdr-less path only when the `herdr` binary
is not on `$PATH`. The owner's decision: a host with herdr installed
must be able to opt out too.

**The setting.** `--herdr` is a global, negatable setting, on by
default, shaped like `--fetch`. `--no-herdr` turns it off. Like every
frit setting, it resolves from four places, most specific first:

1. the flag;
2. `$FRIT_HERDR`;
3. `herdr: false` in `.frit.yml` or `$FRIT_CONFIG`;
4. the user config.

It is a host's choice, so it lives in that chain, not in a
repository's committed `.frit.yml` conventions.

**What off means.** frit never runs the local `herdr` binary. The
runtime's herdr runner is replaced by one that answers "herdr turned
off" without executing anything. The not-found check reads that
answer as absence, so every herdr-less path from phases 1–5 applies
unchanged. Configured remote hosts are still read over ssh. The
setting is this host's, not theirs.

**Wording.** nudge and message refuse naming why: `herdr not
installed` or `herdr turned off (--no-herdr)`, each followed by `; no
pane to reach`.

**RED.** Unit tests, written failing first:

- the off runner and the not-found check;
- `--no-herdr` and `FRIT_HERDR=false` each making a claim stand its
  lane up with git while a recording herdr sees no call;
- the nudge wording.

C24 runs the built frit with a herdr on `$PATH` that records every
call. `start 7 --go --json --no-herdr` hands back in a git lane, and
the herdr log stays empty.

**BDD coverage.** C24 is command-level: one host's setting. The lease
is untouched, so there is no lease-protocol row.

**Docs and skill.** The README and architecture.md name the opt-out.
The CLAUDE.md configuration list is unchanged, since it names the
chain rather than each setting.

**Gate.** C24 passes and fails with the opt-out unwired. `go test
./...`, the coverage gate and golangci-lint are clean.
