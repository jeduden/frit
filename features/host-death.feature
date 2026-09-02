Feature: Host death, suspension, zombies

  Scenarios from the lease-protocol matrix's own "Host death,
  suspension, zombies" section, one per row, tagged with its S-id.
  A scenario still tagged @pending is declared but not yet written.

  @S14
  Scenario: power loss mid-push
    Given "this host" holds the lease for plan 7, bound in its own lane
    When this host claims plan 7
    Then this host resumes its own lease from the persisted token
    When "this host" commits raw work on its own lane and pushes it
    And this host claims plan 7
    Then this host resumes its own lease from origin's fresh tip, not the stale token

  @S15
  Scenario: host dies holding a claim, never back
    Given "elsewhere" holds the lease for plan 7
    When the hold's takeover window has matured
    And this host claims plan 7
    Then this host takes the lease over, epoch 2, child of the stale tip

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

  @S18
  Scenario: zombie re-runs its own claim
    Given "this host" holds the lease for plan 7, bound in its own lane
    When a live agent sits on that lane's own session
    And this host claims plan 7
    Then the claim is refused, naming the lease already held
    And origin's hold is left exactly as it stood
    When the live agent goes quiet
    And this host claims plan 7
    Then this host resumes its own lease from the persisted token

  @S19
  Scenario: zombie pushes to a completed plan
    Given "box-a" holds the lease for plan 7
    When the plan completes and its ref is deleted on origin
    And "box-a" comes back and renews its lease
    Then the renewal fails
    And origin still has no work ref
    When "box-a" pushes its tip raw
    Then origin accepts it
