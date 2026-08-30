---
id: 2608302230
title: Scaffold PLAN.md index includes folder plans
status: "🔲"
summary: >-
  The scaffold PLAN.md catalog seed globs `plan/*.md`, which matches a
  flat `plan/<id>_slug.md` plan but not a folder plan
  `plan/<id>_slug/plan.md`. In a repo that uses the folder shape, every
  folder plan silently drops out of the generated index and the catalog
  regenerates smaller with no error. Add `plan/*/plan.md` to both
  catalog globs so folder plans are indexed too.
model: sonnet
depends-on: []
---
# Scaffold PLAN.md index includes folder plans

## Goal

Make the scaffold [PLAN.md](../internal/scaffold/assets/PLAN.md)
catalog seed index folder plans, so a repo on the folder shape does not
silently lose plans from its generated index.

## Context

[internal/scaffold/assets/PLAN.md](../internal/scaffold/assets/PLAN.md)
seeds a fresh repo's index with two `<?catalog?>` blocks — "In
progress" and "All plans". Both glob:

```yaml
glob:
  - "plan/*.md"
  - "!plan/proto.md"
```

`plan/*.md` matches a flat `plan/<id>_slug.md` plan but not a folder
plan `plan/<id>_slug/plan.md`. Issue #112 reports that on the folder
shape every folder plan drops out of the index with no error.

**Reuse first.** The asset is embedded via `//go:embed assets/PLAN.md`
in [internal/scaffold/scaffold.go](../internal/scaffold/scaffold.go) as
`planIndex` and written verbatim by `WritePlanIndex`. The rendering is
mdsmith's, not frit's, so the fix is one line in each of the two glob
lists in the asset; no Go change is needed. The existing scaffold test
`TestWritePlanIndexWritesTheSeed` asserts the seed bytes are written —
a content assertion on `planIndex` is the natural red/green home.

## Tasks

1. Add `"plan/*/plan.md"` to both catalog `glob:` lists in the scaffold
   PLAN.md asset, keeping `"!plan/proto.md"` in each.
2. Add a test asserting the embedded `planIndex` seed contains the
   folder-plan glob in both catalog blocks (red before the edit, green
   after).

## Execution

| Phase | Title                          | Tier   | Gate                                                                                                             |
| ----- | ------------------------------ | ------ | ---------------------------------------------------------------------------------------------------------------- |
| 1     | Index folder plans in the seed | sonnet | Test asserts both catalog globs include `plan/*/plan.md`; `mdsmith` render of a repo with a folder plan lists it |

## Acceptance Criteria

- [ ] Both catalog blocks in the scaffold PLAN.md glob `plan/*/plan.md`
      alongside `plan/*.md`, each still excluding `plan/proto.md`
- [ ] A test proves the seed indexes folder plans (fails before the
      edit, passes after)
- [ ] A folder plan appears in the rendered index when mdsmith renders
      a repo scaffolded from the seed
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
