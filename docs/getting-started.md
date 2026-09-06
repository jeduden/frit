# Getting started

Point frit at your repositories, then give one the files it reads.

```sh
export FRIT_ROOT=~/git
cd ~/git/myrepo
frit init --mdsmith .   # writes .frit.yml, .mdsmith.yml, plan/proto.md, PLAN.md
```

Write a plan as `plan/<id>_<slug>.md`, or as
`plan/<id>_<slug>/plan.md` with one `phase-N.md` per phase, following
the template in [plan/proto.md](../plan/proto.md). The id is the
creation minute in UTC, from `date -u +%y%m%d%H%M`. Regenerate the
index with `mdsmith fix PLAN.md`, commit both and push. Then:

```sh
mdsmith check .        # the plan passes the schema, the index is current
frit doctor            # no missing Goal, tier or Execution row
frit ready             # the plan is listed: deps done, nobody holds
frit claim <id>        # pushes plan/<id>, stands up a worktree beside the repo
frit board             # shows the hold and, once one runs, the agent
```

`frit start <id> --go` claims and starts an agent in one step;
`frit pick --go` does the same for the best plan nobody holds.
