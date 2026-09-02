Feature: Partitions

  Scenarios from the lease-protocol matrix's own "Partitions" section,
  one per row, tagged with its S-id. A scenario still tagged @pending
  is declared but not yet written.

  @S20
  Scenario: worker partitioned mid-work
    Given "box-a" holds the lease for plan 20
    And "box-a" commits work on the lane it never pushes
    When the network cuts "box-a" off from origin
    And "box-a" renews its lease
    Then the renewal reports the push unconfirmed
    And origin's tip has not moved
    When an observer watches "box-a"'s tip go stale
    Then the window reads the hold stale
    When "box-b" takes the lease over
    And the partition heals for "box-a"
    And "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-b"
    And the error suggests yield
    And "box-a"'s sibling history is left where it was
    And "box-a"'s push of that work is rejected
    And yield parks "box-a"'s work and leaves "box-b"'s takeover untouched

  @S21
  Scenario: push landed during partition
    Given "box-a" holds the lease for plan 21 in a real lane
    And "box-a"'s next push lands on origin but its confirmation is lost
    When "box-a" renews its lease
    Then the renewal reports the push unconfirmed
    And origin's tip has moved past "box-a"'s claim
    When the partition heals for "box-a"
    Then "box-a"'s persisted token still matches its claim
    And "box-a" is recognized as the owner of origin's tip
    And "box-a" resumes at the same epoch

  @S22 @pending
  Scenario: observer partitioned

  @S23 @pending
  Scenario: everyone partitioned, origin up

  @S24 @pending
  Scenario: asymmetric: push ok, fetch fails

  @S25
  Scenario: stale unwind delete after heal
    Given "box-a" holds the lease for plan 25
    When the network cuts "box-a" off from origin
    And "box-a" renews its lease
    Then the renewal reports the push unconfirmed
    When an observer watches "box-a"'s tip go stale
    Then the window reads the hold stale
    When "box-b" takes the lease over
    And the partition heals for "box-a"
    And "box-a" releases from its recorded tip
    Then the release is fenced naming "box-b"
    And origin still holds the takeover
    And the work ref still exists
