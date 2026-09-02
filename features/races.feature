Feature: Races

  Scenarios from the lease-protocol matrix's own "Races" section, one
  per row, tagged with its S-id. A scenario still tagged @pending is
  declared but not yet written.

  @S26
  Scenario: N claimants, one plan
    Given "box-a" holds the lease for plan 7
    When "box-b" claims plan 7
    And "box-c" claims plan 7
    Then "box-b"'s claim loses, naming "box-a" at epoch 1
    And "box-c"'s claim loses, naming "box-a" at epoch 1
    And origin carries one work ref for plan 7

  @S27
  Scenario: rename between two claimants
    Given "box-a" holds the lease for plan 7
    And "box-b" knows the plan file by a new name
    When "box-b" claims plan 7
    Then "box-b"'s claim loses, naming "box-a" at epoch 1
    And origin carries one work ref for plan 7

  @S28
  Scenario: human deletes ref mid-claim
    Given "box-a" holds the lease for plan 7
    And "box-b" claims plan 7
    And "box-b"'s claim loses, naming "box-a" at epoch 1
    When a human deletes the work ref on origin
    And "box-b" retries plan 7
    Then "box-b"'s retry acquires at epoch 1
    And origin's tip is "box-b"'s claim marker
    When "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-b"

  @S29 @pending
  Scenario: release races a loser's read

  @S30
  Scenario: zombie vs new claimant on one branch
    Given "box-a" holds the lease for plan 7
    And "box-a" commits work on the lane it never pushes
    And "box-b" takes the lease over
    Then "box-a"'s raw push is rejected as non-fast-forward

  @S31
  Scenario: orphan report vs sleeping host
    Given "elsewhere" holds the lease for plan 7, bound to a session
    Then origin's orphan report lists plan 7 as neither stale nor deserted
    When the hold's takeover window has matured
    And "elsewhere"'s bound session wakes and answers live
    And this host claims plan 7
    Then the takeover is refused, naming a live agent session
    And "elsewhere"'s lease is renewed by a beat instead of seized

  @S32 @pending
  Scenario: two same-host sessions race
