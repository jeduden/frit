---
n: 1
title: a cloud session holds as its own session id
status: "✅"
result: false
---
RED, each failing on today's code:

1. Resolving the holder prefers an explicit setting, then
   `$CLAUDE_CODE_REMOTE_SESSION_ID`, then the hostname. Today there is
   no setting, and the hostname is all.
2. C26: the built frit, run with a cloud session id and no herdr,
   claims plan 7. Origin's claim marker names the session. Run again
   from that lane as a second session, it refuses the lane as held by
   the first. Today the marker names the hostname, and the second run
   resumes the first session's lease.

GREEN: a global `--holder` setting, read from `$FRIT_HOLDER` or a
config file like the others. Left empty, the run resolves it once,
and every lease transition and the foreign-hold preflight use it in
place of the hostname. The suite unsets the holder's inputs before it
runs, so its fixtures mint under the hostname even inside a cloud
session.

BDD coverage: C26, a command scenario. The lease protocol is
unchanged: a holder is still an opaque name, and only its value
moves.

Gate: C26 on the built frit, red on the old resolution and green on
the new. `go test ./...` and golangci-lint pass.
