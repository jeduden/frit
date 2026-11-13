---
n: 2
title: a park lands where the cloud proxy admits it
status: "✅"
result: true
summary: >-
  A park now writes a branch under frit/rescue/, which a cloud
  session's proxy admits, and old refs/frit/rescue/ refs still list.
  The index skips rescue branches. C27 parks through a remote that
  refuses what the proxy refuses.
---
# Phase 2 result

## Handoff

Every park, from yield, scavenge, reap or a start's leftover, writes
`refs/heads/frit/rescue/<id>/<holder>-<tip>`. `next`, `show` and the
orphans sweep list both namespaces. The ref parser drops a rescue
branch, local or fetched, so no read verb sees one as a lane.

**Verified by.** C27, a command scenario run through godog against
the built frit under a git-only `$PATH`. Its bare remote carries a
`pre-receive` hook refusing any ref outside `refs/heads/` and any
delete. With the old prefix restored, yield fails at the park. With
the new one, the lane is parked and torn down, and origin carries
nothing under `refs/frit/`. Unit tests pin both prefixes in the list
and the sweep, and the parser's skip. `go test ./...` and
golangci-lint are clean.

**Not run.** The real proxy was not probed with a rescue branch.
That it admits one is inferred from its admitting `plan/<id>`
branches.

**Still open.** A scavenge's delete is refused by the proxy, so
`reap --go` from a cloud session reports the refusal. It belongs on a
host that can delete.
