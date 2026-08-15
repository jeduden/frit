package repocfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileMatchesTheCanonicalShape(t *testing.T) {
	p, err := Compile("plan/{id}-*")
	require.NoError(t, err)

	id, ok := p.Match("plan/2608142306-fleet-index")

	assert.True(t, ok)
	assert.Equal(t, int64(2608142306), id)
}

func TestPatternIsAnchoredAtBothEnds(t *testing.T) {
	p, err := Compile("plan/{id}-*")
	require.NoError(t, err)

	// A ref that merely contains the shape must not match, or
	// backup/ and revert/ branches would read as claims.
	_, ok := p.Match("backup/plan/2608142306-fleet-index")
	assert.False(t, ok, "a prefix must not be ignored")

	_, ok = p.Match("plan/2608142306-fleet-index/old")
	assert.False(t, ok, "* stops at a slash")
}

func TestStarDoesNotCrossASlashButDoubleStarDoes(t *testing.T) {
	single, err := Compile("plan/{id}-*")
	require.NoError(t, err)
	_, ok := single.Match("plan/42-a/b")
	assert.False(t, ok)

	double, err := Compile("plan/{id}-**")
	require.NoError(t, err)
	_, ok = double.Match("plan/42-a/b")
	assert.True(t, ok)
}

func TestMatchRequiresDigitsForTheID(t *testing.T) {
	p, err := Compile("plan/{id}-*")
	require.NoError(t, err)

	_, ok := p.Match("plan/wip-something")

	assert.False(t, ok)
}

func TestCompileEscapesRegexMetacharacters(t *testing.T) {
	// A dot in a pattern is a literal dot, not "any character".
	p, err := Compile("v1.{id}")
	require.NoError(t, err)

	_, ok := p.Match("v1x42")
	assert.False(t, ok, "the dot must not act as a wildcard")

	id, ok := p.Match("v1.42")
	assert.True(t, ok)
	assert.Equal(t, int64(42), id)
}

func TestCompileRejectsAPatternWithoutAnID(t *testing.T) {
	_, err := Compile("plan/*")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "{id}")
}

func TestCompileRejectsMoreThanOneID(t *testing.T) {
	_, err := Compile("plan/{id}-{id}")

	require.Error(t, err)
}

func TestCompileRejectsAnEmptyPattern(t *testing.T) {
	_, err := Compile("")

	require.Error(t, err)
}

func TestHoldsTriesEveryPatternInOrder(t *testing.T) {
	h, err := CompileAll([]string{
		"plan/{id}-*",
		"*/plan-{id}-*",
		"plan-{id}-*",
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		branch string
		want   int64
	}{
		{"plan/2608142306-fleet-index", 2608142306},
		{"claude/plan-25-LtXaU", 25},
		{"plan-152-tools-plugin", 152},
	} {
		id, ok := h.Match(tc.branch)
		assert.True(t, ok, tc.branch)
		assert.Equal(t, tc.want, id, tc.branch)
	}
}

func TestHoldsRejectsWhatNoPatternClaims(t *testing.T) {
	h, err := CompileAll([]string{"plan/{id}-*"})
	require.NoError(t, err)

	for _, branch := range []string{
		"main",
		"v0.69",
		"issue-100",
		"backup/pre-rebase-2608142306",
	} {
		_, ok := h.Match(branch)
		assert.False(t, ok, branch)
	}
}

func TestEmptyHoldsMatchNothing(t *testing.T) {
	h, err := CompileAll(nil)
	require.NoError(t, err)

	_, ok := h.Match("plan/2608142306-anything")

	assert.False(t, ok, "a repo declaring no pattern reports no holds")
}

func TestCompileAllReportsWhichPatternIsBad(t *testing.T) {
	_, err := CompileAll([]string{"plan/{id}-*", "no-id-here"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no-id-here")
}

func TestMatchIgnoresAnOverlongIDRatherThanOverflowing(t *testing.T) {
	p, err := Compile("plan/{id}-*")
	require.NoError(t, err)

	_, ok := p.Match("plan/99999999999999999999999-x")

	assert.False(t, ok, "an id that cannot be an int64 is not an id")
}
