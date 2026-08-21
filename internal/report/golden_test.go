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
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/doctor"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
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
		{"ready", goldenReady()},
		{"pick", goldenPick()},
		{"next", goldenNext()},
		{"show", goldenShow()},
		{"find", goldenFind()},
		{"board", goldenBoard()},
		{"open", goldenOpen()},
		{"nudge", goldenNudge()},
		{"claim", goldenClaim()},
		{"release", goldenRelease()},
		{"yield", goldenYield()},
		{"start", goldenStart()},
		{"orphans", goldenOrphans()},
		{"stale", goldenStale()},
		{"doctor", goldenDoctor()},
		{"who", goldenWho()},
		{"init", Init([]string{
			"/fleet/atlas/.frit.yml", "/fleet/atlas/plan/proto.md"})},
		{"skills", Skills([]string{
			"/fleet/atlas/.claude/skills/plan-pick/SKILL.md",
			"/fleet/atlas/.claude/skills/plan-phase/SKILL.md",
		})},
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

// goldenReady pins the readiness shape: a startable plan carrying its
// dependency edges, and a repository frit could not read travelling in
// the same document.
func goldenReady() *ReadyDoc {
	doc := NewReady("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{
		{
			Key: "forge:atlas:2608161810", Repo: "atlas",
			ID: 2608161810, Status: "🔲",
			Title:     "The dispatch ladder",
			Summary:   "From a board to a seeded prompt.",
			Model:     "opus",
			DependsOn: []int64{2608161809},
			Path:      "plan/2608161810_dispatch-ladder.md",
		},
	})
	doc.AddProblem("broken", errors.New("plan/bad.md: no front matter"))

	return doc
}

// goldenPick pins the ranked-candidate shape: a startable plan, and a
// matured takeover — a held plan whose lease has been observed stale,
// carrying the observed age a consumer applies its own threshold to.
func goldenPick() *PickDoc {
	doc := NewPick("/fleet", "forge")
	doc.SetPlans([]discovery.Plan{
		{
			Key: "forge:atlas:2608161809", Repo: "atlas",
			ID: 2608161809, Status: "🔲",
			Title:   "Discovery — what can I start",
			Summary: "The verbs that make dispatch usable.",
			Model:   "opus",
			Path:    "plan/2608161809_discovery-readiness-verbs.md",
		},
		{
			Key: "forge:orrery:7", Repo: "orrery", ID: 7, Status: "🔲",
			Title: "Shader unit tests", Model: "sonnet",
			Path: "plan/7_shader-unit-tests.md",
			Held: true, Holds: []string{"plan/7"},
			Stale: true, StaleFor: 3 * time.Hour,
		},
	})

	return doc
}

// goldenNext pins the next-phase shape: a plan and the first phase of
// it not done.
func goldenNext() *NextDoc {
	doc := NewNext("/fleet", discovery.Plan{
		Key: "forge:atlas:2608161809", Repo: "atlas",
		ID: 2608161809, Status: "🔳",
		Title: "Discovery — what can I start", Model: "opus",
		Path: "plan/2608161809_discovery-readiness-verbs.md",
		Phases: []planmeta.Phase{
			{N: "1", Title: "selectors", Status: "✅"},
			{
				N: "2", Title: "ready", Status: "🔳",
				Tier: "sonnet", Gate: "test ready lists a startable plan",
				HasExecutionRow: true,
				Body:            "Parse the readiness rule and wire it to a verb.",
			},
		},
	})
	doc.SetRescue([]string{"refs/frit/rescue/2608161809/box-a"})

	return doc
}

// goldenShow pins the dependency-walk shape, including an edge frit
// could not resolve to a known plan.
func goldenShow() *ShowDoc {
	doc := NewShow("/fleet", discovery.DepNode{
		Plan: discovery.Plan{
			Key: "forge:atlas:2608161810", Repo: "atlas",
			ID: 2608161810, Status: "🔲", Title: "The dispatch ladder",
			Goal: "Turn a board into a seeded prompt.",
		},
		Found: true,
		Deps: []discovery.DepNode{
			{
				Plan: discovery.Plan{
					Key: "forge:atlas:2608161809", Repo: "atlas",
					ID: 2608161809, Status: "🔳",
					Title: "Discovery — what can I start",
				},
				Found: true,
			},
			{
				Plan:  discovery.Plan{Repo: "atlas", ID: 999},
				Found: false,
			},
		},
	})
	doc.SetRescue([]string{"refs/frit/rescue/2608161810/box-a"})

	return doc
}

// goldenFind pins the search shape: the query echoed, and the matches
// carrying every field a listing does.
func goldenFind() *FindDoc {
	doc := NewFind("/fleet", "forge", "raymarch")
	doc.SetPlans([]discovery.Plan{
		{
			Key: "forge:orrery:12", Repo: "orrery", ID: 12, Status: "✅",
			Title:   "Raymarch the gas giants",
			Summary: "March a ray through the volume.",
			Model:   "opus",
			Path:    "plan/12_raymarch-gas-giants.md",
		},
	})

	return doc
}

// goldenBoard pins the board shape: an in-progress plan held on a lane
// with a live agent, and a not-started plan nobody holds, in the order
// the board ranks them.
func goldenBoard() *BoardDoc {
	doc := NewBoard("/fleet", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:2608161810", Repo: "atlas", ID: 2608161810,
		Status: "🔳", Title: "The dispatch ladder", Model: "opus",
		Held: true, Holds: []string{"plan/2608161810-dispatch"},
	}, "claude", "working")
	doc.AddPlan(discovery.Plan{
		Key: "forge:orrery:7", Repo: "orrery", ID: 7,
		Status: "🔲", Title: "Shader unit tests", Model: "sonnet",
	}, "", "")

	return doc
}

// goldenOpen pins the handoff shape: the plan open resolved and the
// live pane it raised, carrying the branch and agent that prove the
// right lane was focused.
func goldenOpen() *OpenDoc {
	doc := NewOpen("/fleet", "atlas", 2608161810, "The dispatch ladder")
	doc.Focus(herdr.Lane{
		Pane: herdr.Pane{
			Agent: "claude", Status: herdr.StatusWorking,
			PaneID: "wC:p1", Title: "The dispatch ladder",
		},
		Root:   "/fleet/atlas-dispatch",
		Repo:   "atlas",
		Branch: "plan/2608161810-dispatch",
	})

	return doc
}

// goldenNudge pins the dry-run shape: the typed slash command nudge
// composed, the tier the plan declares, and the idle lane it would land
// in — with nothing sent, because --go was not given.
func goldenNudge() *NudgeDoc {
	doc := NewNudge("/fleet", "atlas", 2608161810,
		"The dispatch ladder", "2", "opus",
		"/plan-phase 2608161810 2", false)
	doc.SetTarget("wC:p1")

	return doc
}

// goldenClaim pins the lease shape: the hold branch frit minted for a
// plan and the base commit it was dated against, the branch a consumer
// reads back to learn what it now holds.
func goldenClaim() *ClaimDoc {
	doc := NewClaim("/fleet", "atlas", 2608161810,
		"The dispatch ladder", "plan/2608161810-dispatch-ladder")
	doc.Minted("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0")

	return doc
}

// goldenYield pins the yield shape: a fenced lane's divergence parked
// to a rescue ref and its worktree torn down.
func goldenYield() *YieldDoc {
	doc := NewYield("/fleet", "atlas", 2608161810,
		"The dispatch ladder", "plan/2608161810")
	doc.Parked("refs/frit/rescue/2608161810/box-a")
	doc.Torn()

	return doc
}

// goldenRelease pins the release shape: this lane's own lease ended
// with a release marker.
func goldenRelease() *ReleaseDoc {
	doc := NewRelease("/fleet", "atlas", 2608161810,
		"The dispatch ladder", "plan/2608161810")
	doc.MarkReleased()

	return doc
}

// goldenStart pins the escalation shape: the claim, worktree, agent tier
// and typed prompt start composed, with a note folded into the prompt and
// nothing spawned because --go was not given.
func goldenStart() *StartDoc {
	return NewStart("/fleet", "atlas", 2608161810, "The dispatch ladder",
		StartPlan{
			Phase: "3", Tier: "opus", Kind: "claude",
			Branch: "plan/2608161810-dispatch-ladder",
			Base:   "refs/remotes/origin/main",
			Lane:   "/fleet/atlas-dispatch-ladder",
			Prompt: "/plan-phase 2608161810 3\n\nskip the flaky VRT case",
		}, false)
}

func goldenOrphans() *OrphansDoc {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", found())
	doc.AddRepo("clean", lanes.Orphans{})
	doc.AddProblem("broken", errors.New("no such worktree"))

	return doc
}

// goldenDoctor pins the semantic-gap shape: a plan with a finding, a
// repository doctor could not scan, and a clean repository — the
// three cases the JSON contract keeps distinct from a table, which
// drops the clean one entirely.
func goldenDoctor() *DoctorDoc {
	doc := NewDoctor("/fleet")
	doc.AddFindings("atlas", []doctor.Finding{
		{
			ID: 2608161809, Path: "plan/2608161809_discovery.md",
			Check: "goal",
			Message: `section "## Goal" has no meaningful body content; ` +
				`add paragraph, list, table, or code content, or add ` +
				`"<?allow-empty-section?>" for an intentional empty section`,
		},
	})
	doc.AddProblem("busted", errors.New("no such directory"))

	return doc
}

// goldenWho pins the two shapes a live board must carry: an
// integrated agent resolved to its plan, and an agent frit cannot read
// working outside the convention — kept with an unknown status, no
// plan and no repository rather than dropped or called idle.
func goldenWho() *WhoDoc {
	doc := NewWho("/fleet")
	doc.AddLane(herdr.Lane{
		Pane: herdr.Pane{
			Agent: "claude", Status: herdr.StatusWorking,
			Workspace: "wC", Session: "8e6a81ff-63e8-410c-ac6c",
			PaneID: "wC:p1", Title: "Land the herdr join",
		},
		Root:   "/fleet/atlas",
		Repo:   "atlas",
		Branch: "plan/2608161808-herdr-join",
		PlanID: 2608161808,
	})
	doc.AddLane(herdr.Lane{
		Pane: herdr.Pane{
			Agent: "pi", Status: herdr.StatusUnknown,
			Workspace: "wP", PaneID: "wP:p2", Title: "off the record",
		},
	})

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
