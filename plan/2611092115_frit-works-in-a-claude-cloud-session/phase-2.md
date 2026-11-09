---
n: 2
title: a park lands where the cloud proxy admits it
status: "✅"
result: false
---
A Claude cloud session's git proxy refuses, with HTTP 403, a push
outside `refs/heads/` and any delete. A park pushes to
`refs/frit/rescue/...`, so `yield`, `reap --go` and a start that
parks a leftover failed there. The owner chose to move rescue refs
under `refs/heads/` over refusing teardown in a cloud session.

RED, each failing on today's code:

1. A park names `refs/heads/frit/rescue/<id>/<holder>-<tip>`.
2. A plan's rescue list, and the sweep across plans, read both that
   namespace and the old `refs/frit/rescue/`.
3. The index skips a rescue branch and its remote-tracking copy, so
   a parked plan file is never read as a lane's.
4. C27: the built frit, herdr-less, against a bare remote whose
   `pre-receive` hook refuses what the proxy refuses. A fenced lane's
   `yield` parks and tears the lane down. Origin carries the rescue
   as a branch and nothing under `refs/frit/`.

GREEN: the rescue prefix moves, the reads take both prefixes, and the
ref parser drops rescue branches. Nothing is migrated: an old rescue
stays where it was parked.

BDD coverage: C27, a command scenario. The lease protocol's park rule
is unchanged except for the name, and no S-matrix row spells the
namespace, so none is added.

Out of scope: a scavenge's delete is still refused in a cloud
session. Teardown that deletes belongs on a host that can delete.

Gate: C27 on the built frit, red with the old prefix and green with
the new. `go test ./...` and golangci-lint pass.
