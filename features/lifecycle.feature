Feature: Lifecycle anomalies

  Scenarios from the lease-protocol matrix's own "Lifecycle anomalies" section, one per row, tagged with its S-id. One still tagged @pending is declared
  but not yet written.

  @S50 @pending
  Scenario: plan file renamed after claim

  @S51 @pending
  Scenario: slug collision across plans

  @S52 @pending
  Scenario: plan deleted while claimed

  @S53 @pending
  Scenario: plan id reused

  @S54 @pending
  Scenario: squash-merge, status never ✅

  @S55 @pending
  Scenario: merge + branch auto-delete

  @S56 @pending
  Scenario: local branch deleted by hand

  @S57 @pending
  Scenario: plan re-opened after done

  @S58 @pending
  Scenario: released before the PR merges

  @S59 @pending
  Scenario: status flipped ✅ early by hand

  @S70 @pending
  Scenario: claim dated against an old base

  @S75 @pending
  Scenario: default branch renamed

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
