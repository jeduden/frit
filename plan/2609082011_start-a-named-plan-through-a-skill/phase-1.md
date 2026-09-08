---
n: 1
title: Ship and prove the named-start skill
status: "🔲"
result: false
---
RED: add a bundle test requiring an installed plan-start skill with an
explicit selector, named-start triggers and the JSON handoff contract.
It must fail while the skill is absent. Use a real local origin with
two ready plans, where the requested plan has lower unblock weight.
Install the skill into the fixture with a custom invocation pointing
to the built frit. Execute its example command with the target id,
using fake herdr, and assert only that plan gains a lane and agent.

GREEN: add the canonical asset through the existing skill bundle. Its
main command is `{{frit}} start <selector> --go --json`. A plain
`{{frit}} start <selector>` can preview the composition. Branch on JSON
for refusals and handoff state; after `prompt_dispatched: true`, report
the pane and stop. Describe the selector as required for this skill.
Do not silently pick a different plan or invoke plan-phase locally.

Update plan-pick and plan-drive routing so a named fresh start reaches
plan-start, choosing unspecified work reaches pick, and raising an
existing pane remains drive. Regenerate installed copies with the
repository's `--via "go run ./cmd/frit"` convention. Update the skill
list in development docs. Preserve the installer and refusal contract.

BDD coverage: implement reserved `@C7` and `@C8` in
[commands.feature](../../features/commands.feature). C7 runs the installed
command for the lower-ranked plan; C8 requests a dependency-blocked
plan while another is ready and proves neither starts. Bind steps in a
section-owned test file using the shared world, without changing the
central registry. Remove pending tags and labels when steps pass.
No new `@S<n>` applies: this phase fronts existing dispatch and changes
no claim, takeover or resume protocol. Existing lease scenarios remain
the regression coverage for those transitions.

Gate: run both scenarios without skipping, assert the requested plan,
dispatched pane and refusal from JSON, and count ref and agent effects.
The installed-command smoke must invoke the built frit, not just check
skill text. Test default and custom invocation installs and the dogfood
match. Check routing prose against actual commands, then run the full
Go tests, lint and `mdsmith check .`, including all skill token budgets.
