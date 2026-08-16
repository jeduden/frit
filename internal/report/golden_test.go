package report

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// update rewrites the golden files instead of comparing against them:
// `go test ./internal/report -update`. Read the diff before committing
// it — this contract is what every consumer of frit is written
// against, and a field that quietly changed name has broken them all.
var update = flag.Bool("update", false, "rewrite the golden files")

// TestGoldenShapes pins the JSON of every command.
//
// The fixtures are built by hand rather than by walking a repository,
// so nothing here moves with the clock, the machine, or a temporary
// directory's name. A diff in these files is a change to the contract
// and nothing else.
func TestGoldenShapes(t *testing.T) {
	cases := []struct {
		name string
		doc  any
	}{
		{"repos", goldenRepos()},
		{"plans", goldenPlans()},
		{"orphans", goldenOrphans()},
		{"stale", goldenStale()},
		{"init", Init("/fleet/atlas/.frit.yml")},
		{"version", Version("1.2.3")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, WriteJSON(&out, tc.doc))

			assert.Equal(t, string(golden(t, tc.name, out.Bytes())),
				out.String())
		})
	}
}

// golden returns the recorded shape, writing it first when -update is
// given.
func golden(t *testing.T, name string, got []byte) []byte {
	t.Helper()
	path := filepath.Join("testdata", name+".json")

	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with -update to record this shape")

	return want
}

func goldenRepos() ReposDoc {
	return Repos("/fleet", []discover.Repo{
		{
			Name: "atlas",
			Path: "/fleet/atlas",
			Worktrees: []gitwt.Worktree{
				{Path: "/fleet/atlas", Branch: "main", Head: "a1b2c3d4"},
				{
					Path:   "/fleet/atlas-fleet-index",
					Branch: "plan/2608142306-fleet-index",
					Head:   "e5f6a7b8",
					Locked: true, LockReason: "in use",
				},
				{Path: "/fleet/atlas-empty", Branch: "plan/7-y", Head: zero},
			},
		},
		{
			Name:      "mirror.git",
			Path:      "/fleet/mirror.git",
			Worktrees: []gitwt.Worktree{{Path: "/fleet/mirror.git", Bare: true}},
		},
	})
}

func goldenPlans() *PlansDoc {
	doc := NewPlans("/fleet", "forge")
	doc.AddRepo("atlas", []index.Entry{
		entry(2608142306, planmeta.StatusInProgress),
		entry(7, planmeta.StatusDone),
	})
	doc.AddRepo("quiet", nil)
	doc.AddProblem("broken", errors.New("plan/bad.md: no front matter"))

	return doc
}

func goldenOrphans() *OrphansDoc {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", found())
	doc.AddRepo("clean", lanes.Orphans{})
	doc.AddProblem("broken", errors.New("no such worktree"))

	return doc
}

func goldenStale() *StaleDoc {
	doc := NewStale("/fleet", 30, true)
	doc.AddRepo("atlas", []lanes.Aged{
		{
			Worktree: gitwt.Worktree{
				Path:   "/fleet/atlas-fleet-index",
				Branch: "plan/2608142306-fleet-index",
				Head:   "e5f6a7b8",
			},
			Age: 41*24*time.Hour + 12*time.Hour,
		},
		{
			Worktree: gitwt.Worktree{
				Path:   "/fleet/atlas-herdr-join",
				Branch: "plan/2608161808-herdr-join",
				Head:   "c0ffee11",
			},
			Age: 33 * 24 * time.Hour,
		},
	}, map[string]bool{"/fleet/atlas-herdr-join": true})
	doc.AddRepo("fresh", nil, nil)

	return doc
}
