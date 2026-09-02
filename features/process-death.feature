Feature: Process death, at every lifecycle step

  Scenarios from the lease-protocol matrix's own "Process death, at
  every lifecycle step" section, one per row, tagged with its S-id. A
  scenario still tagged @pending is declared but not yet written.

  @S1 @pending
  Scenario: killed before local ref write

  @S2 @pending
  Scenario: killed after local write, before push

  @S3 @pending
  Scenario: killed mid-push, server committed

  @S4 @pending
  Scenario: killed before worktree creation

  @S5 @pending
  Scenario: killed between worktree and agent start

  @S6 @pending
  Scenario: killed between agent start and prompt

  @S7 @pending
  Scenario: observer saw a claim that then unwound

  @S8 @pending
  Scenario: unwind's remote delete fails

  @S9 @pending
  Scenario: unwind deletes a branch with pushed work

  @S10 @pending
  Scenario: killed mid-phase, work pushed

  @S11 @pending
  Scenario: killed mid-phase, work only local

  @S12 @pending
  Scenario: killed after merge, before status flip

  @S13 @pending
  Scenario: status flipped on branch, not merged
