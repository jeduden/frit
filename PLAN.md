# Plans

## In progress

<?catalog
glob:
  - "plan/*.md"
  - "!plan/proto.md"
where: 'status: "🔳"'
sort: numeric:id
header: |

  | ID  | Model | Title |
  |-----|-------|-------|
row: "| {id} | {model} | [{title}]({filename}) |"
footer: |

empty: |

  Nothing in progress.

?>

| ID         | Model  | Title                                                                                                           |
| ---------- | ------ | --------------------------------------------------------------------------------------------------------------- |
| 2608280653 | sonnet | [A plan may be a folder holding a fixed plan.md, beside flat plan files](plan/2608280653_folder-based-plans.md) |
<?/catalog?>

## All plans

<?catalog
glob:
  - "plan/*.md"
  - "!plan/proto.md"
sort: numeric:id
header: |

  | ID  | Status | Model | Title |
  |-----|--------|-------|-------|
row: "| {id} | {status} | {model} | [{title}]({filename}) |"
footer: |

?>

| ID         | Status | Model  | Title                                                                                                                                  |
| ---------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| 2608142306 | ✅     | opus   | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md)                                 |
| 2608161808 | ✅     | sonnet | [The herdr join — which lane has an agent, live](plan/2608161808_herdr-join-live-presence.md)                                          |
| 2608161809 | ✅     | opus   | [Discovery — what can I start, and what blocks it](plan/2608161809_discovery-readiness-verbs.md)                                       |
| 2608161810 | ✅     | opus   | [The dispatch ladder — from a board to a seeded prompt](plan/2608161810_dispatch-ladder.md)                                            |
| 2608161811 | ✅     | sonnet | [Multi-host — read the sockets, not the socket](plan/2608161811_multi-host-fan-out.md)                                                 |
| 2608171835 | ✅     | opus   | [The claim lease — frit mints the hold, atomically](plan/2608171835_claim-lease.md)                                                    |
| 2608172211 | ✅     | sonnet | [One fleet walk — carry the repo coordinates](plan/2608172211_one-fleet-walk.md)                                                       |
| 2608191811 | ✅     | sonnet | [Report a checkout stranded on a landed branch](plan/2608191811_report-a-stranded-checkout.md)                                         |
| 2608191812 | ✅     | sonnet | [Name who holds a claim when the race is lost](plan/2608191812_name-the-holder-of-a-lost-race.md)                                      |
| 2608192020 | ✅     | sonnet | [Ship agent skills that teach frit's own workflow](plan/2608192020_ship-skills.md)                                                     |
| 2608192045 | ✅     | sonnet | [Enrich next and show so a skill leans on frit's output](plan/2608192045_next-enrichment.md)                                           |
| 2608192121 | ✅     | sonnet | [init scaffolds the plan machinery, not just the config](plan/2608192121_init-scaffolds-plan-machinery.md)                             |
| 2608192233 | ✅     | sonnet | [Harden the claim against squash-merges and shared checkouts](plan/2608192233_claim-watertight.md)                                     |
| 2608192322 | ✅     | sonnet | [A claimed lane is its own worktree, never the shared clone](plan/2608192322_claim-stands-up-its-worktree.md)                          |
| 2608202144 | ✅     | sonnet | [The hold is the work ref, a self-healing lease](plan/2608202144_lease-namespace-claims.md)                                            |
| 2608211210 | ✅     | sonnet | [pick --go selects the top plan and starts it, so a skill needs one verb](plan/2608211210_pick-go-claims-and-starts.md)                |
| 2608211326 | ✅     | sonnet | [The lease protocol completed through every verb](plan/2608211326_lease-protocol-completion.md)                                        |
| 2608211936 | ✅     | sonnet | [A blocked scavenge names its rescue ref and what to do next](plan/2608211936_rescue-conflict-guidance.md)                             |
| 2608212010 | ✅     | sonnet | [Scavenge reads squash-merged work as landed, so it parks nothing that landed](plan/2608212010_squash-aware-landed-evidence.md)        |
| 2608212011 | ✅     | sonnet | [The rescue ref carries its tip, so a park never conflicts](plan/2608212011_content-addressed-rescue-refs.md)                          |
| 2608212203 | ✅     | sonnet | [Only a minted lease is a hold, and a dead session frees it](plan/2608212203_only-a-lease-is-a-hold.md)                                |
| 2608212218 | ✅     | sonnet | [frit reaps the orphans it reports](plan/2608212218_reap-the-orphans.md)                                                               |
| 2608212223 | ✅     | sonnet | [A thin skill fronts every frit verb an agent uses](plan/2608212223_a-skill-fronts-every-verb.md)                                      |
| 2608212236 | ✅     | sonnet | [A --go dispatch reads as a handoff, not a to-do](plan/2608212236_dispatch-reads-as-a-handoff.md)                                      |
| 2608212346 | ✅     | sonnet | [A deserted hold is seen and has a way out](plan/2608212346_deserted-hold-recovery.md)                                                 |
| 2608220940 | ✅     | sonnet | [Scavenge never deletes a branch a worktree still stands on](plan/2608220940_scavenge-spares-a-checked-out-branch.md)                  |
| 2608220941 | ✅     | sonnet | [A fetched remote-tracking ref outrunning main is named](plan/2608220941_stale-default-branch-is-named.md)                             |
| 2608221025 | ✅     | sonnet | [The installer chooses how a laid-down skill invokes frit](plan/2608221025_skills-choose-frit-invocation.md)                           |
| 2608221227 | ✅     | sonnet | [Guard the skill bundle: no drift, tiny, bounded in the binary](plan/2608221227_guard-the-skill-bundle.md)                             |
| 2608221450 | ✅     | sonnet | [Landed evidence reads origin's default branch, not a lagging local one](plan/2608221450_landed-evidence-reads-origins-default.md)     |
| 2608221537 | ✅     | sonnet | [The dispatch ladder starts a phase-less plan instead of demanding --phase](plan/2608221537_dispatch-a-phaseless-plan.md)              |
| 2608221559 | ✅     | sonnet | [Remove the superseded Mint claim path](plan/2608221559_remove-superseded-mint-path.md)                                                |
| 2608221754 | ✅     | sonnet | [The supervisor acts on a lane it does not stand in](plan/2608221754_supervisor-acts-on-a-foreign-lane.md)                             |
| 2608222201 | ✅     | sonnet | [Reap streams per-lane progress, so a long run isn't silent](plan/2608222201_reap-streams-per-lane-progress.md)                        |
| 2608230705 | ✅     | sonnet | [A fresh claim refuses to clobber local work on its own lease branch](plan/2608230705_claim-guards-local-branch-work.md)               |
| 2608230952 | ✅     | sonnet | [A not-matured hold refusal shows how long it has been held](plan/2608230952_not-matured-refusal-shows-how-long-held.md)               |
| 2608231006 | ✅     | sonnet | [A lane owns its lease after its own raw commits advance the branch](plan/2608231006_release-recognizes-own-lane-after-raw-commits.md) |
| 2608231201 | ✅     | sonnet | [Read verbs refresh from origin before reporting landed evidence](plan/2608231201_read-verbs-fetch-before-reporting.md)                |
| 2608251947 | ✅     | sonnet | [frit owns the status-drift evidence plan-sync hand-runs git for](plan/2608251947_frit-owns-status-drift-evidence.md)                  |
| 2608251958 | ✅     | sonnet | [next and show read the held lane's own plan, not the default branch](plan/2608251958_next-show-read-the-held-lane.md)                 |
| 2608252140 | ✅     | sonnet | [A failed handoff tears its lane down, so start leaves no half-built lane](plan/2608252140_failed-handoff-tears-its-lane-down.md)      |
| 2608260639 | ✅     | sonnet | [A dispatched handoff carries the consumer's next action, not a bare prompt](plan/2608260639_dispatch-report-carries-next-action.md)   |
| 2608262155 | ✅     | sonnet | [The herdr pane label names the plan's repo, not the id alone](plan/2608262155_pane-label-names-the-repo.md)                           |
| 2608271957 | ✅     | sonnet | [A stalled git network call cannot hang a frit verb](plan/2608271957_git-calls-cannot-hang.md)                                         |
| 2608272240 | ✅     | sonnet | [nudge treats an unread host as presence unknown, not an absent lane](plan/2608272240_nudge-treats-unread-host-as-presence-unknown.md) |
| 2608280653 | 🔳     | sonnet | [A plan may be a folder holding a fixed plan.md, beside flat plan files](plan/2608280653_folder-based-plans.md)                        |
| 2608281623 | ✅     | sonnet | [A stalled herdr call cannot hang a frit verb](plan/2608281623_herdr-calls-cannot-hang.md)                                             |
| 2608282218 | 🔲     | sonnet | [A timed-out subprocess is killed, not left to linger](plan/2608282218_timed-out-subprocess-is-killed.md)                              |
<?/catalog?>
