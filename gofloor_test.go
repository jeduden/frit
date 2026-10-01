package frit_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// goFloor is the oldest Go language version frit builds with. go.mod's
// go directive is a floor every consumer inherits: `go install` refuses,
// or fetches a newer toolchain, below it. A dependency that raises it —
// the golang.org/x modules moved to 1.26 together — arrives as a quiet
// one-line diff in a dependabot bump, so the floor is pinned here and
// moving it is a decision made in this file, not a merge. Patch
// releases still move freely: a security fix in the standard library
// may raise 1.25.x without changing what frit asks of a consumer.
const goFloor = "1.25"

func TestGoDirectiveHoldsTheFloor(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^go (\d+\.\d+)(\.\d+)?\s*$`).FindSubmatch(data)
	require.NotNil(t, m, "go.mod has no go directive")
	require.Equal(t, goFloor, string(m[1]),
		"go.mod raises the minimum Go version; move goFloor only by decision")
}
