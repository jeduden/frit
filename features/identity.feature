Feature: Identity anomalies

  Scenarios from the lease-protocol matrix's own "Identity anomalies"
  section, one per row, tagged with its S-id. A scenario still
  tagged @pending is declared but not yet written.

  @S45 @pending
  Scenario: two agents, one plan, one host

  @S46 @pending
  Scenario: worktree path reused

  @S47 @pending
  Scenario: worktree debris fails the handoff

  @S48
  Scenario: hostname changes
    Given a held lane holding plan 7 whose marker names "elsewhere" as holder and whose checkout carries the token
    And herdr shows no agent on the lane
    When this machine runs start --go for plan 7
    Then the plan is resumed
    And no takeover marker sits between the held tip and origin's tip

  @S49 @pending
  Scenario: hostname collides

  @S66 @pending
  Scenario: NFS-shared clone across hosts
