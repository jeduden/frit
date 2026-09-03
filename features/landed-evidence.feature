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

  @S59
  Scenario: status flipped ✅ early by hand
    Given a repository with plan 59 hand-flipped to ✅ and plan 60 depending on it
    When ready runs
    Then plan 60 is listed as ready

  @S79
  Scenario: scavenge deletes a branch a worktree still stands on
    Given "box-a" holds the lease for plan 79
    And "box-a" pushes work on the lane
    And a worktree stands on "box-a"'s branch
    When "box-a" scavenges at the observed tip
    Then the work is parked to a rescue ref
    And origin's work ref for the plan is gone
    And "box-a"'s local work ref still resolves at its tip

  @S80
  Scenario: local default branch lags its own fetched remote-tracking ref
    Given a repository whose origin's main has advanced past the local clone's own main
    When board runs
    Then the report names the local default branch lagging

  @S81
  Scenario: unstaffed hold, holder alive on another machine
    Given "box-a" holds the lease for plan 81 bound to a session
    And a herdr fake confirms "box-a"'s bound session alive
    When a fleet-wide reap --go runs
    Then the hold is refused naming a live lease
    And the hold still resolves on origin

  @S82
  Scenario: reaped squash-landed branch carries a follow-up commit
    Given a stranded, landed checkout on plan 82's branch
    When a fleet-wide reap --go runs
    Then the branch is reaped
    And the checkout's own commit is parked to the plan's rescue ref

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

  @S87
  Scenario: read verb reads landed evidence off a checkout unfetched since a PR merged
    Given a checkout unfetched since its plan's lease was deleted upstream
    When board runs with the default fetch
    Then the plan reads as landed, off the board
    When board runs with --no-fetch
    Then the plan reads as held, off the stale local view
