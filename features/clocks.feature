Feature: Clocks

  Scenarios from the lease-protocol matrix's own "Clocks" section, one
  per row, tagged with its S-id. A scenario still tagged @pending is
  declared but not yet written.

  @S33
  Scenario: frozen clock on worker
    Given "box-a" holds the lease for plan 33
    And "box-a"'s commit clock is pinned to one instant
    When "box-a" renews its lease
    And an observer samples the current tip
    And "box-a" renews its lease
    And an observer samples the current tip
    Then the two beats carry the same commit date and different SHAs
    And the window resets to one sample with no void
    And the window reads the hold live

  @S34
  Scenario: clock steps backward
    Given "box-a" holds the lease for plan 34
    When "box-a" renews its lease
    And an observer samples the current tip
    And "box-a"'s commit clock steps years backward
    And "box-a" renews its lease
    And an observer samples the current tip
    Then the tip still moved
    And the window resets to one sample with no void
    And the commit date on the tip is smaller than on its parent

  @S35
  Scenario: clock steps far forward
    Given "box-a" holds the lease for plan 35
    When an observer watches "box-a"'s tip go stale
    Then the window reads the hold stale
    When "box-b" takes the lease over
    Then origin holds the takeover
    When a further observer watches "box-b"'s tip mature by the same span
    Then that span does not read stale once the takeover count backs the threshold off

  @S36 @pending
  Scenario: cross-host clock skew
