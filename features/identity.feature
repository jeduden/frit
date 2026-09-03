Feature: Identity anomalies

  Scenarios from the lease-protocol matrix's own "Identity anomalies"
  section, one per row, tagged with its S-id. A scenario still
  tagged @pending is declared but not yet written.

  @S45
  Scenario: two agents, one plan, one host
    Given "elsewhere" holds plan 7 bound to a session
    And the window has matured for plan 7
    And herdr confirms the session is live
    When this machine runs start --go for plan 7
    Then start refuses, naming the live agent session
    And the holder's own lease is renewed, not seized

  @S46
  Scenario: worktree path reused
    Given "elsewhere" holds plan 7 with its lane's token persisted
    When this machine runs claim for plan 7 from an unrelated directory
    Then claim refuses: already held
    And the plan 7 ref is unchanged

  @S47
  Scenario: worktree debris fails the handoff
    Given plan 7 is unclaimed
    And the agent fails to start and its own teardown leaves debris behind
    When this machine runs start --go for plan 7
    Then start fails and a release marker sits on the branch
    And the error names the worktree and pane left behind

  @S48
  Scenario: hostname changes
    Given a held lane holding plan 7 whose marker names "elsewhere" as holder and whose checkout carries the token
    And herdr shows no agent on the lane
    When this machine runs start --go for plan 7
    Then the plan is resumed
    And no takeover marker sits between the held tip and origin's tip

  @S49
  Scenario: hostname collides
    Given a held lane holding plan 7 whose marker names this host as holder but whose checkout carries no token
    And herdr shows no agent on the lane
    When this machine runs start --go for plan 7
    Then start refuses: already held, not takeable until the window matures
    And the plan is not resumed

  @S66
  Scenario: NFS-shared clone across hosts
    Given this machine holds plan 7 in a lane with its token persisted
    Then the marker's lane trailer is a bare path naming no host
    And the lane's token lives inside that path's git directory
