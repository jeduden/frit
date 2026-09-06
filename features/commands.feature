Feature: Command scenarios

  Scenarios from command-scenarios.md's own catalog, tagged with its
  C-id — a command behavior worth a scenario that is not a
  lease-protocol one.

  @C1
  Scenario: release on a plan nobody ever held
    Given a plan nobody has ever held
    When it is released
    Then the release is a no-op, not a refusal

  @C2
  Scenario: a plan whose work merged is reported as landed
    Given a plan in progress whose work has merged into main
    When frit drift is run
    Then drift reports the plan's work has landed
    And drift names the commit that carries the plan's id

  @C3
  Scenario: yield on a plan nobody holds is a clean no-op
    Given a plan nobody has ever held
    When it is yielded
    Then yield parks nothing and refuses nothing

  @C4
  Scenario: a plan whose final phase landed surfaces as drift
    Given a multi-phase plan in progress whose last phase's commit is on main
    When frit drift is run
    Then drift reports that a commit names the plan's final phase

  @C5
  Scenario: a plan mid-flight or already done raises no drift
    Given a plan mid-flight whose work has not merged into main
    And a plan already marked done
    When frit drift is run
    Then drift raises nothing for the mid-flight plan
    And drift does not list the done plan

  @C6
  Scenario: yielding one's own live lane is refused toward release
    Given a plan freshly claimed by this lane
    When it is yielded
    Then yield refuses it, naming release as the way out
