Feature: Lifecycle anomalies — landed evidence

  The landed-evidence half of the lease-protocol matrix's "Lifecycle
  anomalies" section, one scenario per row, tagged with its S-id: how
  scavenge, reap and the read verbs decide that work has landed, and
  what they refuse to do without that evidence. The claim-and-ref
  half is lifecycle.feature. One still tagged @pending is declared but
  not yet written.

  @S54 @pending
  Scenario: squash-merge, status never ✅

  @S59 @pending
  Scenario: status flipped ✅ early by hand

  @S79 @pending
  Scenario: scavenge deletes a branch a worktree still stands on

  @S80 @pending
  Scenario: local default branch lags its own fetched remote-tracking ref

  @S81 @pending
  Scenario: unstaffed hold, holder alive on another machine

  @S82 @pending
  Scenario: reaped squash-landed branch carries a follow-up commit

  @S83 @pending
  Scenario: origin unreadable while scavenge classifies the ref

  @S84 @pending
  Scenario: local default branch normally lags origin, so it is never authoritative for evidence

  @S85 @pending
  Scenario: `origin/HEAD` unset, so `DefaultRef` falls back to a local default branch

  @S87 @pending
  Scenario: read verb reads landed evidence off a checkout unfetched since a PR merged
