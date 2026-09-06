# Command scenarios: a BDD home for behavior that is not the lease protocol

2026-09-06. Plan 2609052303 phase 3. The lease-protocol matrix in
[lease-protocol.md](lease-protocol.md) catalogs cross-host lease
behavior: claim, release, yield, takeover, and how they read a foreign
hold. Some command behavior worth a scenario is not that — a refusal's
wording, a `--json` shape, a `next_action` a single host computes with
no lease race in sight. This catalog is its home, numbered `C<n>`
rather than `S<n>` so a reader tells the two apart at a glance. It is
kept in bijection with `features/` the same way, by the same gate:
`go test ./internal/scenario`. The procedure to add or write a row is
[docs/development.md](../development.md)'s executable scenario matrix.

| #   | Scenario                                       | Outcome and mechanism                                                                                                                |
| --- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| C1  | release on a plan nobody ever held             | reported as a no-op, not a refusal — nothing to end, one host, no lease race (dispatch)                                              |
| C2  | a plan whose work merged is reported as landed | drift reports landed and names the commit that carries the plan's id — the ancestor-merge signal, read from the command's own output |
