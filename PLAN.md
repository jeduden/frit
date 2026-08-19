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

| ID         | Model  | Title                                                                                        |
| ---------- | ------ | -------------------------------------------------------------------------------------------- |
| 2608192045 | sonnet | [Enrich next and show so a skill leans on frit's output](plan/2608192045_next-enrichment.md) |
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

| ID         | Status | Model  | Title                                                                                                      |
| ---------- | ------ | ------ | ---------------------------------------------------------------------------------------------------------- |
| 2608142306 | ✅     | opus   | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md)     |
| 2608161808 | ✅     | sonnet | [The herdr join — which lane has an agent, live](plan/2608161808_herdr-join-live-presence.md)              |
| 2608161809 | ✅     | opus   | [Discovery — what can I start, and what blocks it](plan/2608161809_discovery-readiness-verbs.md)           |
| 2608161810 | ✅     | opus   | [The dispatch ladder — from a board to a seeded prompt](plan/2608161810_dispatch-ladder.md)                |
| 2608161811 | ✅     | sonnet | [Multi-host — read the sockets, not the socket](plan/2608161811_multi-host-fan-out.md)                     |
| 2608171835 | ✅     | opus   | [The claim lease — frit mints the hold, atomically](plan/2608171835_claim-lease.md)                        |
| 2608172211 | ✅     | sonnet | [One fleet walk — carry the repo coordinates](plan/2608172211_one-fleet-walk.md)                           |
| 2608191811 | ✅     | sonnet | [Report a checkout stranded on a landed branch](plan/2608191811_report-a-stranded-checkout.md)             |
| 2608191812 | ✅     | sonnet | [Name who holds a claim when the race is lost](plan/2608191812_name-the-holder-of-a-lost-race.md)          |
| 2608192020 | ✅     | sonnet | [Ship agent skills that teach frit's own workflow](plan/2608192020_ship-skills.md)                         |
| 2608192045 | 🔳     | sonnet | [Enrich next and show so a skill leans on frit's output](plan/2608192045_next-enrichment.md)               |
| 2608192121 | 🔲     | sonnet | [init scaffolds the plan machinery, not just the config](plan/2608192121_init-scaffolds-plan-machinery.md) |
<?/catalog?>
