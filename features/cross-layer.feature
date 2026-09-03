Feature: Cross-layer: herdr and frit disagree

  Scenarios from the lease-protocol matrix's own "Cross-layer: herdr
  and frit disagree" section, one per row, tagged with its S-id. A
  scenario still tagged @pending is declared but not yet written.

  @S60
  Scenario: herdr down at claim time
    Given plan 7 is unclaimed
    And herdr is unreachable
    When this machine claims plan 7
    Then the claim is refused: worktree not stood up
    And the lease is released, not left standing
    When herdr becomes reachable
    And this machine claims plan 7
    Then it claims clean at the next epoch

  @S61
  Scenario: herdr down at observation
    Given "elsewhere" holds plan 7 bound to a session
    And the window has matured for plan 7
    And herdr is unreachable
    When "box-b" claims plan 7 over the stale hold
    Then the claim is refused: worktree not stood up
    And a takeover at epoch 2 sits on the stale tip
    And the veto never fired

  @S62 @pending
  Scenario: host unreachable, agents pushing

  @S63 @pending
  Scenario: pane alive, lease released

  @S64
  Scenario: branch repurposed by hand
    Given this machine holds plan 7 in a lane with its token persisted
    When origin's work ref for plan 7 is deleted and repurposed by "elsewhere"
    Then the lane's release is refused and origin's tip is untouched
    And the lane's claim reports the plan already held, never resumed

  @S65 @pending
  Scenario: herdr restarts, loses panes

  @S72 @pending
  Scenario: claim and start race on one host

  @S73
  Scenario: prompt fails after agent start
    Given plan 7 is unclaimed
    And the agent starts but its prompt fails
    When this machine runs start --go for plan 7
    Then start fails and a release marker sits on the branch
    And the agent was started before the failure
    And the worktree it stood up is torn down

  @S74 @pending
  Scenario: same plan id in two repos

  @S76 @pending
  Scenario: pane gone before the window matures

  @S77 @pending
  Scenario: deserted lane on its own host

  @S86
  Scenario: a live lane's own raw commits advance the branch past its persisted token
    Given this machine holds plan 7 in a lane with its token persisted
    And two raw commits are pushed on top of the lane
    When the lane runs claim for plan 7
    Then it is resumed and the beat's parent is the raw tip
    When a takeover at a new epoch lands on plan 7
    And the lane runs release for plan 7
    Then it is refused and the takeover stands
