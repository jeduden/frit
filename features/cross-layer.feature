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

  @S62
  Scenario: host unreachable, agents pushing
    Given "elsewhere" holds plan 7 bound to a session
    And the window has matured for plan 7
    And herdr is unreachable
    And the holder pushes a raw commit on top of the held tip
    When "box-b" claims plan 7 over the stale hold
    Then claim refuses: already held
    And the refusal names the window not yet matured

  @S63
  Scenario: pane alive, lease released
    Given this machine holds plan 7 in a lane bound to a session, with its token persisted
    And herdr confirms the lane's own session is live
    And a takeover bound to a session at a new epoch lands on plan 7
    When the lane runs release for plan 7
    Then it is refused and the takeover stands

  @S64
  Scenario: branch repurposed by hand
    Given this machine holds plan 7 in a lane with its token persisted
    When origin's work ref for plan 7 is deleted and repurposed by "elsewhere"
    Then the lane's release is refused and origin's tip is untouched
    And the lane's claim reports the plan already held, never resumed

  @S65
  Scenario: herdr restarts, loses panes
    Given "elsewhere" holds plan 7 bound to a session
    And the window has matured for plan 7
    And herdr shows no agent on the lane
    When "box-b" claims plan 7 over the stale hold
    Then it takes over cleanly at the next epoch
    And the veto never fired

  @S72
  Scenario: claim and start race on one host
    Given plan 7 is unclaimed
    When claim and start both race to mint plan 7
    Then one wins and the loser's refusal names the winning lane

  @S73
  Scenario: prompt fails after agent start
    Given plan 7 is unclaimed
    And the agent starts but its prompt fails
    When this machine runs start --go for plan 7
    Then start fails and a release marker sits on the branch
    And the agent was started before the failure
    And the worktree it stood up is torn down

  @S74
  Scenario: same plan id in two repos
    Given plan 7 is unclaimed in "atlas" and in "forge"
    When this machine claims plan 7 in "atlas" and in "forge"
    Then both are claimed with no collision, and the lanes and panes carry the repo

  @S76
  Scenario: pane gone before the window matures
    Given a held lane holding plan 7 whose marker names "elsewhere" as holder and names no session
    And herdr shows no agent on the lane
    When this machine runs start --go for plan 7
    Then the plan is resumed
    And no takeover marker sits between the held tip and origin's tip

  @S77
  Scenario: deserted lane on its own host
    Given this machine holds plan 7 in a lane bound to a session, with its token persisted
    And a takeover bound to a session at a new epoch lands on plan 7
    And herdr shows no agent on the lane
    When the lane runs start --go for plan 7
    Then start refuses and names yield
    And it is refused and the takeover stands

  @S86
  Scenario: a live lane's own raw commits advance the branch past its persisted token
    Given this machine holds plan 7 in a lane with its token persisted
    And two raw commits are pushed on top of the lane
    When the lane runs claim for plan 7
    Then it is resumed and the beat's parent is the raw tip
    When a takeover at a new epoch lands on plan 7
    And the lane runs release for plan 7
    Then it is refused and the takeover stands
