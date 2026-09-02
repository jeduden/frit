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
    When "box-a" retries the claim
    And "box-b" claims plan 7
    Then "box-a"'s retry is refused: the local branch diverges
    And "box-b" wins the lease at epoch 1

  @S3
  Scenario: killed mid-push, server committed
    Given "box-a" holds the lease for plan 7 with its token persisted in its own worktree
    When "box-a" runs claim for plan 7 from its own worktree
    Then the claim resumes instead of refusing

  @S4
  Scenario: killed before worktree creation
    Given "box-a" has claimed plan 7 but its worktree was never stood up
    When "box-a" runs claim for plan 7
    Then the claim is refused, not resumed
    When the takeover window has matured for plan 7
    And "box-b" runs claim for plan 7
    Then the claim takes the lease over

  @S5
  Scenario: killed between worktree and agent start
    Given "box-a" holds the lease for plan 7
    When "box-a" runs board
    Then board shows plan 7 held with no session

  @S6
  Scenario: killed between agent start and prompt
    Given "box-a" holds the lease for plan 7 with a session bound
    And herdr confirms no live agent on that session
    And the takeover window has matured for plan 7
    When "box-b" runs claim for plan 7
    Then the claim takes the lease over

  @S7
  Scenario: observer saw a claim that then unwound
    Given "box-a" holds the lease for plan 7
    And the observer has matured a window on "box-a"'s tip
    When "box-b" takes the lease over
    And the observer resets the window on the new tip
    Then the observation restarts fresh on the new tip

  @S8
  Scenario: unwind's remote delete fails
    Given "box-a" holds the lease for plan 7
    When "box-a"'s handoff unwinds and releases the lease
    Then origin still carries the work ref for plan 7
    And the work ref's tip is a release marker

  @S9
  Scenario: unwind deletes a branch with pushed work
    Given "box-a" holds the lease for plan 7
    And "box-a" pushes a work commit on the lane
    When the ref is scavenged
    Then the pushed work is parked to a rescue ref, not lost
    And the work ref is deleted from origin

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

  @S12
  Scenario: killed after merge, before status flip
    Given "box-a" holds the lease for plan 7
    And "box-a" pushes a work commit on the lane
    And that work is squash-landed on origin's default branch
    When the ref is scavenged
    Then nothing is parked, because the work already landed
    And the work ref is deleted from origin

  @S13
  Scenario: status flipped on branch, not merged
    Given "box-a" holds the lease for plan 7
    And "box-a" pushes a work commit on the lane
    And plan 7 is marked done on the branch, but origin's default branch is untouched
    When the landed evidence for plan 7 is read
    Then the work reads as unlanded, because evidence is origin's default branch only
