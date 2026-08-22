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

Nothing in progress.
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

| ID         | Status | Model  | Title                                                                                                                           |
| ---------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------- |
| 2608142306 | ✅     | opus   | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md)                          |
| 2608161808 | ✅     | sonnet | [The herdr join — which lane has an agent, live](plan/2608161808_herdr-join-live-presence.md)                                   |
| 2608161809 | ✅     | opus   | [Discovery — what can I start, and what blocks it](plan/2608161809_discovery-readiness-verbs.md)                                |
| 2608161810 | ✅     | opus   | [The dispatch ladder — from a board to a seeded prompt](plan/2608161810_dispatch-ladder.md)                                     |
| 2608161811 | ✅     | sonnet | [Multi-host — read the sockets, not the socket](plan/2608161811_multi-host-fan-out.md)                                          |
| 2608171835 | ✅     | opus   | [The claim lease — frit mints the hold, atomically](plan/2608171835_claim-lease.md)                                             |
| 2608172211 | ✅     | sonnet | [One fleet walk — carry the repo coordinates](plan/2608172211_one-fleet-walk.md)                                                |
| 2608191811 | ✅     | sonnet | [Report a checkout stranded on a landed branch](plan/2608191811_report-a-stranded-checkout.md)                                  |
| 2608191812 | ✅     | sonnet | [Name who holds a claim when the race is lost](plan/2608191812_name-the-holder-of-a-lost-race.md)                               |
| 2608192020 | ✅     | sonnet | [Ship agent skills that teach frit's own workflow](plan/2608192020_ship-skills.md)                                              |
| 2608192045 | ✅     | sonnet | [Enrich next and show so a skill leans on frit's output](plan/2608192045_next-enrichment.md)                                    |
| 2608192121 | ✅     | sonnet | [init scaffolds the plan machinery, not just the config](plan/2608192121_init-scaffolds-plan-machinery.md)                      |
| 2608192233 | ✅     | sonnet | [Harden the claim against squash-merges and shared checkouts](plan/2608192233_claim-watertight.md)                              |
| 2608192322 | ✅     | sonnet | [A claimed lane is its own worktree, never the shared clone](plan/2608192322_claim-stands-up-its-worktree.md)                   |
| 2608202144 | ✅     | sonnet | [The hold is the work ref, a self-healing lease](plan/2608202144_lease-namespace-claims.md)                                     |
| 2608211210 | ✅     | sonnet | [pick --go selects the top plan and starts it, so a skill needs one verb](plan/2608211210_pick-go-claims-and-starts.md)         |
| 2608211326 | ✅     | sonnet | [The lease protocol completed through every verb](plan/2608211326_lease-protocol-completion.md)                                 |
| 2608211936 | 🔲     | sonnet | [A blocked scavenge names its rescue ref and what to do next](plan/2608211936_rescue-conflict-guidance.md)                      |
| 2608212010 | 🔲     | sonnet | [Scavenge reads squash-merged work as landed, so it parks nothing that landed](plan/2608212010_squash-aware-landed-evidence.md) |
| 2608212011 | 🔲     | sonnet | [The rescue ref carries its tip, so a park never conflicts](plan/2608212011_content-addressed-rescue-refs.md)                   |
| 2608212203 | ✅     | sonnet | [Only a minted lease is a hold, and a dead session frees it](plan/2608212203_only-a-lease-is-a-hold.md)                         |
| 2608212218 | 🔲     | sonnet | [frit reaps the orphans it reports](plan/2608212218_reap-the-orphans.md)                                                        |
| 2608212223 | 🔲     | sonnet | [A thin skill fronts every frit verb an agent uses](plan/2608212223_a-skill-fronts-every-verb.md)                               |
| 2608212236 | 🔲     | sonnet | [A --go dispatch reads as a handoff, not a to-do](plan/2608212236_dispatch-reads-as-a-handoff.md)                               |
| 2608212346 | 🔲     | sonnet | [A deserted hold is seen and has a way out](plan/2608212346_deserted-hold-recovery.md)                                          |
<?/catalog?>
