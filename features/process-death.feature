Feature: Process death, at every lifecycle step

  Scenarios from the lease-protocol matrix's own "Process death, at
  every lifecycle step" section, one per row, tagged with its S-id. A
  scenario still tagged @pending is declared but not yet written.

  @S1
  Scenario: killed before local ref write
    Given "box-a" has a claimable plan 7
    When "box-a"'s claim dies before writing anything
    Then origin has no work ref for plan 7
    And "box-a" retries and acquires the lease at epoch 1

  @S2
  Scenario: killed after local write, before push
    Given "box-a" mints a local claim it never pushes for plan 7
    When "box-b" claims plan 7
    Then "box-b" wins the lease at epoch 1
    And "box-a"'s retry is refused: the local branch diverges

  @S3 @pending
  Scenario: killed mid-push, server committed

  @S4 @pending
  Scenario: killed before worktree creation

  @S5 @pending
  Scenario: killed between worktree and agent start

  @S6 @pending
  Scenario: killed between agent start and prompt

  @S7
  Scenario: observer saw a claim that then unwound
    Given "box-a" holds the lease for plan 7
    And the observer has matured a window on "box-a"'s tip
    When "box-b" takes the lease over
    And the observer resets the window on the new tip
    Then the observation restarts fresh on the new tip

  @S8 @pending
  Scenario: unwind's remote delete fails

  @S9 @pending
  Scenario: unwind deletes a branch with pushed work

  @S10
  Scenario: killed mid-phase, work pushed
    Given "box-a" holds the lease for plan 7
    And "box-a" pushes a work commit on the lane
    When "box-b" takes over the current lease
    Then "box-b"'s takeover is a child of the tip that actually reached origin
    And "box-a"'s pushed work is in "box-b"'s takeover history

  @S11
  Scenario: killed mid-phase, work only local
    Given "box-a" holds the lease for plan 7
    And "box-a" commits work on the lane it never pushes
    When "box-b" takes the lease over
    Then "box-b"'s takeover is a child of the tip that actually reached origin
    And "box-a"'s local-only work is in no history on origin

  @S12 @pending
  Scenario: killed after merge, before status flip

  @S13 @pending
  Scenario: status flipped on branch, not merged
