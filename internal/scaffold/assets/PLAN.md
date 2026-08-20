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

empty: |

  No plans yet.

?>

No plans yet.
<?/catalog?>
