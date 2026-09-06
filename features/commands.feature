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
