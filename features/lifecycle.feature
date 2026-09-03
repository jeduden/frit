Feature: Lifecycle anomalies — claims and refs

  The claim-and-ref half of the lease-protocol matrix's "Lifecycle
  anomalies" section, one scenario per row, tagged with its S-id: a
  plan renamed, deleted, reused or re-opened, a ref gone or dated
  against an old base. The landed-evidence half is
  landed-evidence.feature. A scenario still tagged @pending is
  declared but not yet written.

  @S50
  Scenario: plan file renamed after claim
    Given "box-a" holds the lease for plan 7
    When the plan file is renamed on main and pushed
    And "box-b" acquires the lease for plan 7
    Then "box-b" loses to the live lease
    And origin carries exactly one refs/heads/plan/* ref, with no slug in its name

  @S51
  Scenario: slug collision across plans
    Given plans 7 and 8 share a title
    When "box-a" acquires the lease for plan 7
    And "box-a" acquires the lease for plan 8
    Then origin holds refs/heads/plan/7 and refs/heads/plan/8, two refs, neither naming the shared title

  @S52 @pending
  Scenario: plan deleted while claimed

  @S53
  Scenario: plan id reused
    Given plan 7 is done and its lease is released
    When a different plan's file replaces it under the same id 7
    And the released ref is scavenged by evidence
    Then origin carries no plan/7 ref
    And frit claims plan 7 fresh at epoch 1

  @S55
  Scenario: merge + branch auto-delete
    Given plan 7 is merged into main with its branch already auto-deleted
    Then frit claims plan 7 fresh at epoch 1

  @S56
  Scenario: local branch deleted by hand
    Given "box-a" holds the lease for plan 7
    When "box-a" deletes its local branch by hand
    And "box-b" acquires the lease for plan 7
    Then "box-b" loses to the live lease
    When "box-a" comes back and renews its lease
    Then the local branch is restored at the renewed tip

  @S57
  Scenario: plan re-opened after done
    Given plan 7 is done and its lease is released
    When the plan file is marked done and then re-opened
    And the released ref is scavenged by evidence
    Then origin carries no plan/7 ref
    And frit claims plan 7 fresh at epoch 1

  @S58 @pending
  Scenario: released before the PR merges

  @S70
  Scenario: claim dated against an old base
    Given a claimable plan 7
    And origin's main moves past the clone's last fetch
    When frit claims plan 7
    Then the claim marker's base names origin's current main

  @S75
  Scenario: default branch renamed
    Given a clone of an origin whose default branch is "main"
    When origin renames its default branch to "trunk"
    And the clone re-reads origin's HEAD
    Then DefaultRef answers "refs/remotes/origin/trunk"
    When origin renames its default branch to "quay"
    And the clone re-reads origin's HEAD
    Then DefaultRef answers "refs/remotes/origin/quay"
