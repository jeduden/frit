Feature: Lifecycle anomalies — landed evidence

  The landed-evidence half of the lease-protocol matrix's "Lifecycle
  anomalies" section, one scenario per row, tagged with its S-id: how
  scavenge, reap and the read verbs decide that work has landed, and
  what they refuse to do without that evidence. The claim-and-ref
  half is lifecycle.feature. A scenario still tagged @pending is
  declared but not yet written.

  @S54
  Scenario: squash-merge, status never ✅
    Given "box-a" holds the lease for plan 54
    And "box-a" pushes work on the lane
    And "box-b" clones the repository
    When "box-b" squash-merges the work onto the default branch
    And "box-b" scavenges at the observed tip
    Then origin's work ref for the plan is gone
    And nothing is parked

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

  @S83
  Scenario: origin unreadable while scavenge classifies the ref
    Given "box-a" holds the lease for plan 83
    When origin becomes unreadable
    And "box-a" scavenges at the observed tip
    Then the scavenge fails naming the read
    And "box-a"'s local work ref still resolves at its tip

  @S84
  Scenario: local default branch normally lags origin, so it is never authoritative for evidence
    Given "box-a" holds the lease for plan 84
    And "box-a" pushes work on the lane
    And "box-b" clones the repository
    When "box-b" squash-merges the work onto the default branch
    Then "box-a"'s local main is behind the default branch
    And the work reads landed for "box-a"
    And a scavenge by "box-a" parks nothing

  @S85
  Scenario: `origin/HEAD` unset, so `DefaultRef` falls back to a local default branch
    Given "box-a" holds the lease for plan 85
    And "box-a" pushes work on the lane
    And "box-b" clones the repository
    When "box-b" squash-merges the work onto the default branch
    Then DefaultRef for "box-a" answers refs/remotes/origin/main, not refs/heads/main
    And the work reads landed for "box-a"

  @S87 @pending
  Scenario: read verb reads landed evidence off a checkout unfetched since a PR merged
