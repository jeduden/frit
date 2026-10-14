---
n: 7
title: who on a headless host still lists the configured hosts' lanes
status: "✅"
result: false
---
A review of the docs found that `who` reads presence with the read
that stops at any local herdr failure. On a host with no herdr it
reported the missing herdr and never asked the configured `--hosts`.
A lane live on another host was left out, even though start's gates
already read past the missing herdr since C22. The owner chose to
close the gap.

**The change.** `who` reads presence the way start's gates do: a herdr
not installed here, or turned off, is read past, and the hosts are
still asked. The missing herdr still travels as a problem, so the
list reads as partial, never as "nobody live here". An installed herdr
that does not answer still stops the read before any host is asked,
as before.

**RED.** Unit tests, written failing first:

- with herdr not installed and `--hosts box`, `who` lists box's lane
  on its plan and carries one herdr problem;
- with no cache path, the unread host is carried beside the missing
  herdr.

C25 runs the built frit under a git-only `$PATH` with a fake ssh
answering for host `box`. `who --json --hosts box` lists plan 7's
lane from box and carries the herdr problem.

**BDD coverage.** C25 is command-level: one verb's read on one host.
No lease moves, so there is no lease-protocol row.

**Docs.** The README's herdr bullet, architecture.md's presence line
and reaping.md's `who` line say the hosts are still read.

**Gate.** C25 passes, and fails before the change. `go test ./...`,
the coverage gate and golangci-lint are clean.
