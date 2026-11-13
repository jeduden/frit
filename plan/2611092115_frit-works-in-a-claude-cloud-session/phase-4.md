---
n: 4
title: a renamed holder keeps its own lane
status: "✅"
result: false
---
A second review found a lock-out in the check that refuses a lane
another holder has. That check read the oldest claim on the lane, not
the marker governing the lease now. Two holders lost their own lane
to it:

- one whose name changed since it claimed, the case phase 1 creates
  for every lease minted as `vm` before upgrading;
- one that took a lane over, since the oldest claim still named the
  holder it took from.

Each was told the lane was held by the old name, by every verb run
from the lane, with no way through but naming the plan.

The check now reads the newest marker on the lane, so a renewal or a
takeover moves the lane to the holder it names. A claim written
before the lease protocol carries no such marker and is still read
by its own `host:` line.

This amends decision 6 of phase 3. A lease minted as `vm` is not left
to the takeover window when its own session renews it: the renewal
names the session, and the lane is its own again.

BDD coverage: this is command-level behavior, and no row covered it,
so C28 is new. A host claims as `vm`, then holds as a cloud session,
renews from the lane, and releases from it.

Gate: C28 on the built frit, red before the change and green after.
