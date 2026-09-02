Feature: Host death, suspension, zombies

  Scenarios from the lease-protocol matrix's own "Host death,
  suspension, zombies" section, one per row, tagged with its S-id.
  A scenario still tagged @pending is declared but not yet written.

  @S14 @pending
  Scenario: power loss mid-push

  @S15 @pending
  Scenario: host dies holding a claim, never back

  @S16
  Scenario: host resurrected days later
    Given "box-a" holds the lease for plan 7
    And "box-a" commits work on the lane it never pushes
    And "box-b" takes the lease over
    When "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-b"
    And the error suggests yield
    And "box-a"'s sibling history is left where it was
    And "box-a"'s push of that work is rejected
    And yield parks "box-a"'s work and leaves "box-b"'s takeover untouched

  @S17
  Scenario: suspended weeks, plan re-claimed
    Given "box-a" holds the lease for plan 7
    And "box-a" commits work on the lane it never pushes
    And "box-b" takes the lease over
    And "box-b" releases the lease
    And "box-c" claims the released plan
    Then the re-claim lands at epoch 3, a child of the release marker
    When "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-c"
    And yield parks "box-a"'s work and leaves "box-c"'s re-claim untouched

  @S18 @pending
  Scenario: zombie re-runs its own claim

  @S19
  Scenario: zombie pushes to a completed plan
    Given "box-a" holds the lease for plan 7
    When the plan completes and its ref is deleted on origin
    And "box-a" comes back and renews its lease
    Then the renewal fails
    And origin still has no work ref
    When "box-a" pushes its tip raw
    Then origin accepts it
