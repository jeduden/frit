Feature: Storage anomalies

  Scenarios from the lease-protocol matrix's own "Storage anomalies"
  section, one per row, tagged with its S-id. A scenario still
  tagged @pending is declared but not yet written.

  @S37
  Scenario: work ref hand-deleted
    Given "box-a" holds the lease for plan 37
    When a person deletes the work ref on origin
    And "box-a" comes back and renews its lease
    Then the renewal is refused
    And origin's work ref is gone
    And "box-a"'s local ref still points at its recorded tip

  @S38
  Scenario: work ref hand-force-pushed
    Given "box-a" holds the lease for plan 38
    When a person force-pushes a marker forged to name "mallory" onto the work ref
    And "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "mallory"
    And the error suggests yield

  @S39
  Scenario: work ref force-pushed backward
    Given "box-a" holds the lease for plan 39
    And "box-a" comes back and renews its lease
    When a person force-pushes the work ref back to "box-a"'s first tip
    And "box-b" takes the lease over
    Then origin still holds the takeover
    When "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-b"

  @S40
  Scenario: remote GC reaps deleted markers
    Given "box-a" holds the lease for plan 40
    And "box-a" pushes a work commit on the lane
    When the ref is scavenged
    And a person runs "git gc --prune=now" on origin
    Then the pushed work is parked to a rescue ref, not lost

  @S41
  Scenario: remote rewritten or migrated
    Given "box-a" holds the lease for plan 41
    When origin is replaced by a fresh remote carrying only main
    And every machine is pointed at the new remote
    And "box-a" comes back and renews its lease
    Then the renewal is refused
    When "box-b" acquires the lease on the new remote
    Then "box-b" acquired at epoch 1 on the new remote

  @S42 @pending
  Scenario: two remotes, split coordination

  @S43 @pending
  Scenario: origin URL edited mid-lifecycle

  @S44 @pending
  Scenario: fork-based flow

  @S67
  Scenario: `fetch --prune` races a read
    Given "box-a" holds the lease for plan 67
    And "box-b" takes the lease over
    When "box-a" renews its lease while a person's "fetch --prune" races it
    Then the renewal is fenced, naming "box-b"
    And the renewal read origin exactly once to classify the loss

  @S68
  Scenario: default branch force-pushed
    Given "box-a" has a checkout whose branch lands plan 68 on origin's main
    When a person force-pushes origin's main to a fresh commit, same content, no merge
    Then "box-a"'s branch is no longer an ancestor of origin's main
    And reap still reaps "box-a"'s checkout, landed by its status glyph alone

  @S69
  Scenario: marker body forged
    Given "box-a" holds the lease for plan 69
    When a person pushes a beat marker forged to name "box-a" onto the work ref
    And "box-a" comes back and renews its lease
    Then the renewal is fenced, naming "box-a"

  @S71
  Scenario: origin restored from backup
    Given "box-a" holds the lease for plan 71
    And a mirror backup of origin is taken
    And "box-a" renews its lease again
    When origin is restored from the backup
    And "box-a" renews its lease again
    Then the renewal is fenced, naming "box-a"
    When "box-a" re-reads origin's tip and renews from it
    Then the renewal is a plain win
    When "box-a" renews its lease again
    Then the renewal is a plain win

  @S78
  Scenario: two parks from one lane, different tips
    Given "box-a" holds the lease for plan 78
    And "box-a" pushes a work commit on the lane
    When the ref is scavenged
    Then the pushed work is parked to a rescue ref, not lost
    When "box-a" acquires the lease again
    And "box-a" pushes a work commit on the lane
    And the ref is scavenged
    Then the pushed work is parked to a rescue ref, not lost
    And orphans lists both tips as rescued for "box-a"'s lane
