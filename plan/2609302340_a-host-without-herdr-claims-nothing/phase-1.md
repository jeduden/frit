---
n: 1
title: Refuse before the mint when herdr is not present
status: "✅"
result: false
---
RED, each failing on today's code:

1. `herdr.Missing` answers the not-found cause for a runner whose call
   fails with exec's not-found error, and nil for a dial error or an
   answer. Driven through the real runner under `WithTimeout` with an
   empty `$PATH`, it still finds the cause.
2. `claim` with a not-installed herdr refuses with `herdr not found;
   nothing claimed`, carries the raw error as a herdr problem under
   `--json`, and leaves no `plan/<id>` on origin or locally. A plan
   held elsewhere still refuses as already held.
3. `start --go` refuses the same way and pushes nothing; a dry-run
   `start` does not refuse. `pick --go` over two ready plans refuses
   and pushes neither.

GREEN: probe herdr once, just before the mint, in claim and in
`buildStart` under `--go`. Refuse only when the executable is not
found. Report the refusal to `pick --go` as not skippable.

BDD coverage: command-level, one host, no lease race — so `@C<n>`, not
`@S<n>`. C15 (claim) and C16 (`pick --go`) run the built frit under a
`$PATH` holding git alone, so the real exec lookup fails, not a fake
runner. The lease protocol's S60 and S61 are unchanged: they drive an
installed-but-unreachable herdr, which this gate lets through.

Gate: C15 fails on the old code with the issue's own `worktree not
stood up` output, and passes after. `go test ./...` and golangci-lint
pass.
