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

| #   | Scenario                                                              | Outcome and mechanism                                                                                                                                                                        |
| --- | --------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| C1  | release on a plan nobody ever held                                    | reported as a no-op, not a refusal — nothing to end, one host, no lease race (dispatch)                                                                                                      |
| C2  | a plan whose work merged is reported as landed                        | drift reports landed and names the commit that carries the plan's id — the ancestor-merge signal, read from the command's own output                                                         |
| C3  | yield on a plan nobody holds                                          | parks nothing and refuses nothing — the clean no-op, distinct from a fenced lane's refusal (yieldNothingLocal)                                                                               |
| C4  | a plan whose final phase landed surfaces as drift                     | drift reports that a commit names the plan's final phase — the phase-level signal, read from the command's own output                                                                        |
| C5  | a plan mid-flight or already done raises no drift                     | drift raises nothing for unmerged work and never lists a done plan — the restraint a developer trusts before a status flip                                                                   |
| C6  | yielding one's own live lane                                          | refused toward `release` — a freshly claimed lease is the live holder, not fenced, so yield never discards it (StillHeldError)                                                               |
| C7  | starting a named plan that is not top ranked                          | pending — the installed plan-start command starts only the selected plan and reports the dispatched pane; plan 2609082011, issue #185                                                        |
| C8  | a named start refuses without choosing another plan                   | pending — the installed plan-start command reports the selected plan's unmet dependency and leaves another ready plan unheld; plan 2609082011, issue #185                                    |
| C9  | a repo's own proto.md widens the tier vocabulary                      | doctor accepts a tier its own proto.md names, no matching Go change needed (doctor.Scan, badTier)                                                                                            |
| C10 | that added tier ranks correctly through frit next                     | next's --json phase.tier reports the added tier, not a built-in neighbor it would otherwise always lose to (index.Build, Resume)                                                             |
| C11 | claiming from a lane whose branch diverged                            | refused naming the branch and both tips, not "already held" — resume's divergence is carried in the report (LeaseDivergesError)                                                              |
| C12 | starting from a lane whose branch diverged                            | the same refusal, not a fault — `pick --go` skips it like a lost race rather than aborting its walk (startRefusable)                                                                         |
| C13 | an ask answered from its lane                                         | `message --ask --go` sends an envelope naming the reply; `reply` from the lane records the answer with no pane or ref touched; `board --json` reads it answered (ask.Pose, ask.Answer)       |
| C15 | claiming on a host with no herdr                                      | git stands the lane up on the minted tip, its token kept and no session bound; release from it ends the lease — the old refusal is withdrawn (createLaneWorktree)                            |
| C16 | pick --go on a host with no herdr                                     | claims the top candidate, stands its lane up with git and hands the prompt back — handoff `handed-back`; the next candidate is left alone (MarkHandedBack)                                   |
| C17 | release and yield on a decorated hold with no lease                   | both refuse it as held by the decorated branch, in the same words and way out — never "nothing holds it", never "live" (refuseUnproved, #204)                                                |
| C18 | start --go on a host with no herdr                                    | handed back with no pane: started, the prompt not dispatched; run again from the lane, it resumes on its token and hands back again (standUpHeadless)                                        |
| C19 | messaging a lane on a host with no herdr                              | refused naming herdr not installed, never "unreachable", and no herdr problem carried — no pane can exist to reach (herdrNotInstalledRefusal)                                                |
| C20 | yield from a fenced lane on a host with no herdr                      | parks the lane's work, then removes its worktree with git from the primary checkout, which itself is never removed (tearDownHeadlessLane)                                                    |
| C21 | a harness session publishes its lane through its own branch           | lane work pushed to `plan/<id>` and copied to the harness branch; once it merges, the plan reads unheld — existing landed evidence                                                           |
| C22 | start --go on a host with no herdr, a lane live on another host       | the configured hosts are still read past the missing local herdr, so the live lane refuses the start, carrying no herdr problem — never a second lane handed back (fleetPresencePastMissing) |
| C23 | start --go on a host with no herdr, a leftover worktree on the branch | its unlanded work is parked and the worktree left standing, the refusal naming path, rescue ref and `frit reap`; `reap --go` then removes it (leftoverKept)                                  |
| C24 | a host with herdr installed opts out with --no-herdr                  | runs as a herdr-less host — a git lane, the prompt handed back — and never runs the local herdr (herdr.Off)                                                                                  |
