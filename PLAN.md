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

| ID         | Model | Title                                                                                       |
| ---------- | ----- | ------------------------------------------------------------------------------------------- |
| 2608161810 | opus  | [The dispatch ladder — from a board to a seeded prompt](plan/2608161810_dispatch-ladder.md) |
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

| ID         | Status | Model  | Title                                                                                                  |
| ---------- | ------ | ------ | ------------------------------------------------------------------------------------------------------ |
| 2608142306 | ✅     | opus   | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md) |
| 2608161808 | ✅     | sonnet | [The herdr join — which lane has an agent, live](plan/2608161808_herdr-join-live-presence.md)          |
| 2608161809 | ✅     | opus   | [Discovery — what can I start, and what blocks it](plan/2608161809_discovery-readiness-verbs.md)       |
| 2608161810 | 🔳     | opus   | [The dispatch ladder — from a board to a seeded prompt](plan/2608161810_dispatch-ladder.md)            |
| 2608161811 | 🔲     | sonnet | [Multi-host — read the sockets, not the socket](plan/2608161811_multi-host-fan-out.md)                 |
| 2608171835 | ✅     | opus   | [The claim lease — frit mints the hold, atomically](plan/2608171835_claim-lease.md)                    |
<?/catalog?>
