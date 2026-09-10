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

| #   | Scenario                                            | Outcome and mechanism                                                                                                                                     |
| --- | --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| C1  | release on a plan nobody ever held                  | reported as a no-op, not a refusal — nothing to end, one host, no lease race (dispatch)                                                                   |
| C2  | a plan whose work merged is reported as landed      | drift reports landed and names the commit that carries the plan's id — the ancestor-merge signal, read from the command's own output                      |
| C3  | yield on a plan nobody holds                        | parks nothing and refuses nothing — the clean no-op, distinct from a fenced lane's refusal (yieldNothingLocal)                                            |
| C4  | a plan whose final phase landed surfaces as drift   | drift reports that a commit names the plan's final phase — the phase-level signal, read from the command's own output                                     |
| C5  | a plan mid-flight or already done raises no drift   | drift raises nothing for unmerged work and never lists a done plan — the restraint a developer trusts before a status flip                                |
| C6  | yielding one's own live lane                        | refused toward `release` — a freshly claimed lease is the live holder, not fenced, so yield never discards it (StillHeldError)                            |
| C7  | starting a named plan that is not top ranked        | pending — the installed plan-start command starts only the selected plan and reports the dispatched pane; plan 2609082011, issue #185                     |
| C8  | a named start refuses without choosing another plan | pending — the installed plan-start command reports the selected plan's unmet dependency and leaves another ready plan unheld; plan 2609082011, issue #185 |
| C9  | a repo's own proto.md widens the tier vocabulary    | doctor accepts a tier its own proto.md names, no matching Go change needed (doctor.Scan, badTier)                                                         |
| C10 | that added tier ranks correctly through frit next   | next's --json phase.tier reports the added tier, not a built-in neighbor it would otherwise always lose to (index.Build, Resume)                          |
