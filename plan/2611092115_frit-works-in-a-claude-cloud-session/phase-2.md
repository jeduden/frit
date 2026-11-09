---
n: 2
title: teardown answers the cloud proxy's refusals
status: "🔲"
result: false
---
A Claude cloud session's git proxy refuses, with HTTP 403, a push to
`refs/frit/*` and a branch delete. A park and a scavenge each need
one, so `yield`, `reap --go` and a start that parks a leftover fail
with git's bare error.

Waiting on the owner's choice:

- *Refuse clearly.* A cloud session is a worker. The append-only
  transitions it needs all pass the proxy. Teardown is supervisor
  work, so a refused push is reported as the remote refusing it, with
  the verb to run from a host that can push. No protocol change.
- *Move rescue refs under `refs/heads/`.* A cloud session could then
  park, though a delete is still refused there. Every host must learn
  the new names.

BDD coverage: a command scenario. Its bare remote refuses
`refs/frit/*` and deletes through a `pre-receive` hook. The shape it
pins follows the choice above.
