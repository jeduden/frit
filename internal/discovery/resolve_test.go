package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fleet is a small hand-built set the resolver tests select from.
func fleet() []Plan {
	return []Plan{
		{
			Key: "forge:atlas:2608161809", Repo: "atlas", ID: 2608161809,
			Title:    "Discovery — what can I start",
			Path:     "plan/2608161809_discovery-readiness-verbs.md",
			Branches: []string{"plan/2608161809-discovery"},
		},
		{
			Key: "forge:atlas:2608161810", Repo: "atlas", ID: 2608161810,
			Title: "The dispatch ladder",
			Path:  "plan/2608161810_dispatch-ladder.md",
		},
		{
			Key: "forge:orrery:7", Repo: "orrery", ID: 7,
			Title: "Shader unit tests",
			Path:  "plan/7_shader-unit-tests.md",
		},
	}
}

func TestResolveByExactID(t *testing.T) {
	got, err := Resolve("2608161810", fleet())

	require.NoError(t, err)
	assert.Equal(t, "The dispatch ladder", got.Title)
}

func TestResolveBySlugFragmentInTitle(t *testing.T) {
	got, err := Resolve("shader-unit", fleet())

	require.NoError(t, err)
	assert.Equal(t, int64(7), got.ID)
}

func TestResolveBySlugFragmentInBranch(t *testing.T) {
	got, err := Resolve("discovery", fleet())

	require.NoError(t, err)
	assert.Equal(t, int64(2608161809), got.ID)
}

func TestResolveBySlugFragmentInPath(t *testing.T) {
	got, err := Resolve("readiness", fleet())

	require.NoError(t, err)
	assert.Equal(t, int64(2608161809), got.ID)
}

func TestResolveIsCaseInsensitive(t *testing.T) {
	got, err := Resolve("SHADER", fleet())

	require.NoError(t, err)
	assert.Equal(t, int64(7), got.ID)
}

// TestResolveAmbiguousSlugListsCandidates is the acceptance criterion:
// a fragment matching more than one plan prints the candidates rather
// than guessing.
func TestResolveAmbiguousSlugListsCandidates(t *testing.T) {
	// "2608161" is not an id anyone allocated, so it falls through to a
	// slug match — and appears in both atlas plan paths.
	_, err := Resolve("2608161", fleet())

	var amb *Ambiguous
	require.ErrorAs(t, err, &amb)
	assert.Len(t, amb.Candidates, 2)
	assert.Contains(t, err.Error(), "dispatch ladder")
	assert.Contains(t, err.Error(), "2608161810")
}

// TestResolveAmbiguousIDAcrossRepos: the same id in two repositories is
// an ambiguity, not a silent pick — the fleet key spans repos.
func TestResolveAmbiguousIDAcrossRepos(t *testing.T) {
	plans := append(fleet(), Plan{
		Repo: "mirror", ID: 7, Title: "A coincidental seven",
	})

	_, err := Resolve("7", plans)

	var amb *Ambiguous
	require.ErrorAs(t, err, &amb)
	assert.Len(t, amb.Candidates, 2)
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("raymarch", fleet())

	require.ErrorIs(t, err, ErrNotFound)
}

// TestResolveFallsFromIDToSlug: a numeric selector that matches no id
// still gets a chance at a slug match rather than failing outright.
func TestResolveFallsFromIDToSlug(t *testing.T) {
	plans := []Plan{{ID: 42, Title: "About 99 red balloons"}}

	got, err := Resolve("99", plans)

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.ID)
}
