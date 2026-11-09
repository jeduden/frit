---
n: 1
title: a cloud session holds as its own session id
status: "✅"
result: true
summary: >-
  A cloud session now holds as its session id, and --holder names any
  host's holder through the settings chain. A second session run from
  the first one's lane refuses it as foreign instead of resuming it.
  C26 covers it.
---
# Phase 1 result

## Handoff

Every lease marker a run mints, and the preflight that refuses a
foreign hold, now name the resolved holder instead of the hostname.
The board's host column still shows the hostname, since it keys
plans and not leases.

**Verified by.** C26, a command scenario run through godog against
the built frit under a git-only `$PATH`. With the old resolution
forced back in, the marker step fails. With the marker step dropped,
the second session's run from the lane succeeds, so it resumed a
lease that was not its own. With the new resolution, both steps pass.
Unit tests pin the three branches of the resolution and the hostname
fallback for a runtime built without one. `go test ./...` and
golangci-lint are clean.

**Still open.** A cloud session whose container is reclaimed loses
its lane and its token, while the session id, and so the holder,
lives on. Whether a resumed session should pick its own hold back up
is not decided here. The window still frees the plan.
