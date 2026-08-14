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

?>

| ID         | Model | Title                                                                                                  |
| ---------- | ----- | ------------------------------------------------------------------------------------------------------ |
| 2608142306 | opus  | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md) |
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

| ID         | Status | Model | Title                                                                                                  |
| ---------- | ------ | ----- | ------------------------------------------------------------------------------------------------------ |
| 2608142306 | 🔳     | opus  | [The fleet index — discover every repo, worktree and branch](plan/2608142306_fleet-index-discovery.md) |
<?/catalog?>
